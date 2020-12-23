package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/BurntSushi/toml"
	"gonum.org/v1/gonum/graph/encoding/dot"
)

func main() {
	var (
		onlyExpr   string
		removeExpr string
		showExpr   string
		hideExpr   string

		conf     config
		confPath string
	)

	flag.StringVar(&confPath, "conf", "", "")
	flag.StringVar(&onlyExpr, "only", "", "")
	flag.StringVar(&removeExpr, "remove", "", "")
	flag.StringVar(&showExpr, "show", "", "")
	flag.StringVar(&hideExpr, "hide", "", "")
	flag.Parse()

	if len(confPath) > 0 {
		_, err := toml.DecodeFile(confPath, &conf)
		if err != nil {
			log.Fatal(err)

			return
		}
	}

	if len(onlyExpr) > 0 {
		conf.RuleSet = append(conf.RuleSet, configRuleSet{
			Action:    string(actRemove),
			Direction: string(dirExclude),
			Keyword:   onlyExpr,
		})
	}

	if len(removeExpr) > 0 {
		conf.RuleSet = append(conf.RuleSet, configRuleSet{
			Action:    string(actRemove),
			Direction: string(dirInclude),
			Keyword:   removeExpr,
		})
	}

	if len(showExpr) > 0 {
		conf.RuleSet = append(conf.RuleSet, configRuleSet{
			Action:    string(actHide),
			Direction: string(dirExclude),
			Keyword:   showExpr,
		})
	}

	if len(hideExpr) > 0 {
		conf.RuleSet = append(conf.RuleSet, configRuleSet{
			Action:    string(actHide),
			Direction: string(dirInclude),
			Keyword:   hideExpr,
		})
	}

	rs, err := conf.Build()
	if err != nil {
		log.Fatal(err)
	}

	bufi, err := ioutil.ReadAll(bufio.NewReader(os.Stdin))
	if err != nil {
		log.Fatal(err)
	}

	dst := newDotGraph()
	if err := dot.Unmarshal(bufi, dst); err != nil {
		log.Fatal(err)
	}

	for _, r := range rs {
		for nodes := dst.Nodes(); nodes.Next(); {
			r.Apply(dst, nodes.Node())
		}
	}

	bufo, err := dot.Marshal(dst, "root", "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(bufo))
}
