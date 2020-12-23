package main

import (
	"log"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding"
	"gonum.org/v1/gonum/graph/simple"
)

type dotGraph struct {
	*simple.DirectedGraph
}

func newDotGraph() *dotGraph {
	return &dotGraph{
		DirectedGraph: simple.NewDirectedGraph(),
	}
}

func (g *dotGraph) NewNode() graph.Node {
	n := g.DirectedGraph.NewNode()

	return &dotNode{
		Node:  n,
		Name:  "",
		Attrs: make(map[string]string),
	}
}

func (g *dotGraph) linkNodes(n graph.Node, fromNodes, toNodes graph.Nodes) {
	dn, ok := n.(*dotNode)
	if !ok {
		return
	}

	for fromNodes.Next() {
		fn, ok := fromNodes.Node().(*dotNode)
		if !ok {
			continue
		}

		for toNodes.Next() {
			tn, ok := toNodes.Node().(*dotNode)
			if !ok {
				continue
			}

			if g.HasEdgeBetween(fn.ID(), tn.ID()) {
				continue
			}

			if fn.ID() == tn.ID() {
				continue
			}

			log.Println("Link", dn.Name, "|", fn.Name, "-", tn.Name)
			g.SetEdge(&sublinkEdge{Edge: g.DirectedGraph.NewEdge(tn, fn)})
		}
	}
}

func (g *dotGraph) HideNode(n graph.Node) {
	dn, ok := n.(*dotNode)
	if !ok {
		return
	}

	g.linkNodes(dn, g.From(n.ID()), g.To(n.ID()))
	g.linkNodes(dn, g.To(n.ID()), g.From(n.ID()))
	g.linkNodes(dn, g.From(n.ID()), g.From(n.ID()))
	g.linkNodes(dn, g.To(n.ID()), g.To(n.ID()))
	g.RemoveNode(n.ID())
}

type sublinkEdge struct {
	graph.Edge
}

func (e *sublinkEdge) Attributes() []encoding.Attribute {
	return []encoding.Attribute{
		{Key: "color", Value: "gray"},
		{Key: "dir", Value: "none"},
	}
}

type dotNode struct {
	graph.Node

	Attrs map[string]string
	Name  string
}

func (n *dotNode) SetDOTID(id string) {
	n.Name = id
}

func (n *dotNode) DOTID() string {
	return n.Name
}

func (n *dotNode) SetAttribute(attr encoding.Attribute) error {
	n.Attrs[attr.Key] = attr.Value

	return nil
}

func (n *dotNode) Attributes() []encoding.Attribute {
	attrs := make([]encoding.Attribute, 0, len(n.Attrs))
	for k, v := range n.Attrs {
		attrs = append(attrs, encoding.Attribute{Key: k, Value: v})
	}

	return attrs
}
