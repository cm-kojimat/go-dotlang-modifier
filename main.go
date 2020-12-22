package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"gonum.org/v1/gonum/graph/formats/dot"
	"gonum.org/v1/gonum/graph/formats/dot/ast"
)

var (
	errUnsupportAction = errors.New("unsupport action")
	errUnsupportExpr   = errors.New("unsupport expr")
	errDeleteStmt      = errors.New("delete statement")
)

func main() {
	var (
		include    string
		exclude    string
		config     ruleConfigSet
		configPath string
	)

	flag.StringVar(&include, "include", "", "")
	flag.StringVar(&exclude, "exclude", "", "")
	flag.StringVar(&configPath, "config", "", "")
	flag.Parse()

	if configPath != "" {
		_, err := toml.DecodeFile(configPath, &config)
		if err != nil {
			log.Fatal(err)

			return
		}
	}

	rs, err := config.Build()
	if err != nil {
		log.Fatal(err)

		return
	}

	if exclude != "" {
		ifm := includeFilter{Keyword: regexp.MustCompile(exclude)}.Match

		rs = append(rs, deleteRule{Filter: nodeFilter{Filter: ifm}.Match}.Apply)
		rs = append(rs, deleteRule{Filter: edgeFilter{Filter: ifm}.Match}.Apply)
	}

	if include != "" {
		ifm := includeFilter{Keyword: regexp.MustCompile(include)}.Match
		efm := excludeFilter{Filter: ifm}.Match
		attrs := map[string]string{"color": "red"}

		rs = append(rs, deleteRule{Filter: nodeFilter{Filter: efm}.Match}.Apply)
		rs = append(rs, modLabelRule{Filter: nodeFilter{Filter: ifm}.Match, Attrs: attrs}.Apply)
		rs = append(rs, deleteRule{Filter: edgeFilter{Filter: efm}.Match}.Apply)
		rs = append(rs, modLabelRule{Filter: edgeFilter{Filter: ifm}.Match, Attrs: attrs}.Apply)
	}

	dast, err := dot.Parse(os.Stdin)
	if err != nil {
		log.Fatal(err)

		return
	}

	for _, g := range dast.Graphs {
		err := walk(g, rs.Apply)
		if err != nil {
			log.Fatal(err)

			return
		}

		fmt.Println(g)
	}
}

func walk(x interface{}, f func(ast.Stmt) error) error {
	switch xt := x.(type) {
	case *ast.Graph:
		for _, xtx := range xt.Stmts {
			err := walk(xtx, f)
			if err != nil {
				return err
			}
		}

		return nil
	case *ast.Subgraph:
		if err := f(xt); err != nil {
			return err
		}

		xs := xt.Stmts
		xss := len(xs)
		d := 0

		for i := 0; i < xss; i++ {
			xtx := xs[i-d]

			err := walk(xtx, f)
			if errors.Is(err, errDeleteStmt) {
				xs = append(xs[:i-d], xs[i-d+1:]...)
				d++

				continue
			}

			if err != nil {
				return err
			}
		}

		xt.Stmts = xs

		return nil
	case ast.Stmt:
		return f(xt)
	default:
		return nil
	}
}

type ruleConfigSet struct {
	Config []ruleConfig `toml:"rule"`
}

func (cs ruleConfigSet) Build() (ruleSet, error) {
	rs := make(ruleSet, 0, len(cs.Config))

	for _, rc := range cs.Config {
		r, err := rc.Build()
		if err != nil {
			return rs, err
		}

		rs = append(rs, r)
	}

	return rs, nil
}

type ruleConfig struct {
	Filter filterConfig      `toml:"filter"`
	Action string            `toml:"action"`
	Attrs  map[string]string `toml:"attr"`
}

func (c ruleConfig) Build() (rule, error) {
	f, err := c.Filter.Build()
	if err != nil {
		return nil, err
	}

	switch c.Action {
	case "DELETE":
		r := deleteRule{Filter: f}

		return r.Apply, nil

	case "MOD_ATTR":
		r := modLabelRule{Filter: f, Attrs: c.Attrs}

		return r.Apply, nil
	}

	return nil, fmt.Errorf("%w: %s", errUnsupportAction, c.Action)
}

type filterConfig struct {
	Key     string `toml:"key"`
	Keyword string `toml:"Keyword"`
	Expr    string `toml:"expr"`
}

func (c filterConfig) Build() (filterFunc, error) {
	keys := strings.Split(c.Key, ".")

	f := includeFilter{
		Keys:    keys,
		Keyword: regexp.MustCompile(c.Keyword),
	}

	expr := strings.Split(c.Expr, ".")

	return buildFilterByExpr(f.Match, expr)
}

func buildFilterByExpr(f filterFunc, expr []string) (filterFunc, error) {
	if len(expr) == 0 {
		return f, nil
	}

	switch expr[0] {
	case "node":
		return buildFilterByExpr(nodeFilter{Filter: f}.Match, expr[1:])
	case "edge":
		return buildFilterByExpr(edgeFilter{Filter: f}.Match, expr[1:])
	case "exclude":
		return buildFilterByExpr(excludeFilter{Filter: f}.Match, expr[1:])
	case "":
		return f, nil

	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportExpr, expr[0])
	}
}

type rule func(ast.Stmt) error

type ruleSet []rule

func (rs ruleSet) Apply(x ast.Stmt) error {
	for _, r := range rs {
		if err := r(x); err != nil {
			return err
		}
	}

	return nil
}

type filterFunc func(ast.Stmt) bool

type includeFilter struct {
	Keys    []string
	Keyword *regexp.Regexp
}

func (f includeFilter) Match(x ast.Stmt) bool {
	return f.Keyword.MatchString(fetchByKeys(f.Keys, x))
}

type excludeFilter struct {
	Filter filterFunc
}

func (f excludeFilter) Match(x ast.Stmt) bool {
	return !f.Filter(x)
}

type nodeFilter struct {
	Filter filterFunc
}

func (f nodeFilter) Match(x ast.Stmt) bool {
	switch xt := x.(type) {
	case *ast.NodeStmt:
		return f.Filter(xt)
	default:
		return false
	}
}

type edgeFilter struct {
	Filter filterFunc
}

func (f edgeFilter) Match(x ast.Stmt) bool {
	switch xt := x.(type) {
	case *ast.EdgeStmt:
		return f.Filter(xt)
	default:
		return false
	}
}

func fetchByKeys(keys []string, x fmt.Stringer) string {
	if len(keys) == 0 || keys[0] == "" {
		return x.String()
	}

	switch xt := x.(type) {
	case *ast.NodeStmt:
		switch keys[0] {
		case "id":
			return xt.Node.ID
		case "port":
			return fetchByKeys(keys[1:], xt.Node.Port)
		default:
			for _, a := range xt.Attrs {
				if a.Key == keys[0] {
					return a.Val
				}
			}

			return ""
		}

	case *ast.Port:
		switch keys[0] {
		case "id":
			return xt.ID
		case "compass_point":
			return fetchByKeys(keys[1:], xt.CompassPoint)
		default:
			return ""
		}

	case *ast.EdgeStmt:
		switch keys[0] {
		case "from":
			return fetchByKeys(keys[1:], xt.From)
		case "to":
			return fetchByKeys(keys[1:], xt.To)
		default:
			for _, a := range xt.Attrs {
				if a.Key == keys[0] {
					return a.Val
				}
			}

			return ""
		}

	case ast.Vertex:
		return xt.String()

	case *ast.Edge:
		switch keys[0] {
		case "vertex":
			return fetchByKeys(keys[1:], xt.Vertex)
		case "to":
			return fetchByKeys(keys[1:], xt.To)
		default:
			return ""
		}

	default:
		return ""
	}
}

type deleteRule struct {
	Filter filterFunc
}

func (r deleteRule) Apply(x ast.Stmt) error {
	if !r.Filter(x) {
		return nil
	}

	return errDeleteStmt
}

type modLabelRule struct {
	Filter filterFunc
	Attrs  map[string]string
}

func (r modLabelRule) Apply(x ast.Stmt) error {
	if !r.Filter(x) {
		return nil
	}

	switch xt := x.(type) {
	case *ast.NodeStmt:
		for k, v := range r.Attrs {
			xt.Attrs = append(xt.Attrs, &ast.Attr{Key: k, Val: v})
		}

		return nil

	case *ast.EdgeStmt:
		for k, v := range r.Attrs {
			xt.Attrs = append(xt.Attrs, &ast.Attr{Key: k, Val: v})
		}

		return nil

	default:
		return nil
	}
}
