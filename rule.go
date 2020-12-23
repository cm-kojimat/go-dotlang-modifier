package main

import (
	"errors"
	"fmt"
	"log"
	"regexp"

	"gonum.org/v1/gonum/graph"
)

type matcher func(string) bool

func notMatcher(m matcher) matcher { return func(s string) bool { return !m(s) } }

type actionType string

const (
	actRemove  = actionType("REMOVE")
	actHide    = actionType("HIDE")
	actModAttr = actionType("MOD_ATTR")
)

type action func(g *dotGraph, n graph.Node)

func removeAction(g *dotGraph, n graph.Node) {
	dn, ok := n.(*dotNode)
	if !ok {
		return
	}

	log.Println("Remove", dn.Name)
	g.RemoveNode(dn.ID())
}

func hideAction(g *dotGraph, n graph.Node) {
	dn, ok := n.(*dotNode)
	if !ok {
		return
	}

	log.Println("Hide", dn.Name)
	g.HideNode(dn)
}

type modAttrActor struct {
	Attrs map[string]string
}

func (a modAttrActor) Action(g *dotGraph, n graph.Node) {
	dn, ok := n.(*dotNode)
	if !ok {
		return
	}

	for k, v := range a.Attrs {
		dn.Attrs[k] = v
	}
}

type direction string

const (
	dirInclude = direction("include")
	dirExclude = direction("exclude")
)

type ruler struct {
	Matcher matcher
	Action  action
}

func (r ruler) Apply(g *dotGraph, n graph.Node) {
	dn, ok := n.(*dotNode)
	if !ok {
		return
	}

	if !r.Matcher(dn.Name) {
		return
	}

	r.Action(g, n)
}

var (
	errConfigUnmatchDirection = errors.New("unmatch direction")
	errConfigUnmatchAction    = errors.New("unmatch action")
)

type configRuleSet struct {
	Action    string            `toml:"action"`
	Direction string            `toml:"direction"`
	Keyword   string            `toml:"keyword"`
	Attrs     map[string]string `toml:"attr"`
}

func (rs configRuleSet) BuildMatcher() (matcher, error) {
	m, err := regexp.Compile(rs.Keyword)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regexp: %w", err)
	}

	switch direction(rs.Direction) {
	case dirExclude:
		return notMatcher(m.MatchString), nil
	case "", dirInclude:
		return m.MatchString, nil
	default:
		return nil, errConfigUnmatchDirection
	}
}

func (rs configRuleSet) BuildAction() (action, error) {
	switch actionType(rs.Action) {
	case actRemove:
		return removeAction, nil
	case actHide:
		return hideAction, nil
	case actModAttr:
		return modAttrActor{Attrs: rs.Attrs}.Action, nil
	default:
		return nil, errConfigUnmatchAction
	}
}

func (rs configRuleSet) Build() (ruler, error) {
	m, err := rs.BuildMatcher()
	if err != nil {
		return ruler{}, err
	}

	a, err := rs.BuildAction()
	if err != nil {
		return ruler{}, err
	}

	return ruler{Matcher: m, Action: a}, nil
}

type config struct {
	RuleSet []configRuleSet `toml:"rule"`
}

func (c config) Build() ([]ruler, error) {
	rss := make([]ruler, 0, len(c.RuleSet))

	for _, crs := range c.RuleSet {
		rs, err := crs.Build()
		if err != nil {
			return nil, err
		}

		rss = append(rss, rs)
	}

	return rss, nil
}
