package dag

import (
	"context"
	"encoding/json"
	"errors"
)

type Graph struct {
	nodes map[string]*node
	edges map[string]map[string]bool // adjacency list

}

func New() *Graph {
	return &Graph{
		nodes: make(map[string]*node),
		edges: make(map[string]map[string]bool),
	}
}

type GraphNode struct {
	Name  string   `json:"name"`
	Input string   `json:"input"`
	After []string `json:"after"`
}

func NewJson(buf []byte) *Graph {
	var g = New()
	var nodes struct {
		Steps []GraphNode `json:"setps"`
	}
	json.Unmarshal(buf, &nodes)
	for _, v := range nodes.Steps {
		g.AddNode(v.Name, v.Input)
	}
	for _, v := range nodes.Steps {
		for _, a := range v.After {
			g.AddEdge(a, v.Name)
		}
	}
	return g
}

type node struct {
	name   string
	propmt string
	after  []*node
	before []*node
}

func (s *Graph) AddNode(name, propmt string) {
	s.nodes[name] = &node{name: name, propmt: propmt}
	s.edges[name] = make(map[string]bool)
}

func (s *Graph) AddEdge(from, to string) {
	s.nodes[from].after = append(s.nodes[from].after, s.nodes[to])
	s.nodes[to].before = append(s.nodes[to].before, s.nodes[from])
	s.edges[from][to] = true
}

func (s *Graph) sort() (lst []string, e error) {
	inDegree := make(map[string]int)
	for node := range s.nodes {
		inDegree[node] = 0
	}
	// 计算入度
	for u := range s.edges {
		for v := range s.edges[u] {
			inDegree[v]++
		}
	}
	queue := []string{}
	for node, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, node)
		}
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		lst = append(lst, u)

		for v := range s.edges[u] {
			inDegree[v]--
			if inDegree[v] == 0 {
				queue = append(queue, v)
			}
		}
	}
	if len(lst) != len(s.nodes) {
		return nil, errors.New("cycle detected: not a DAG")
	}
	return
}

func (s *Graph) IsEmpty() bool {
	return len(s.nodes) == 0
}

func (s *Graph) Run(ctx context.Context, f func(context.Context, string, string) error) (e error) {
	ctx = SetResult(ctx, map[string]string{})
	lst, e := s.sort()
	if e != nil {
		return
	}

	for _, v := range lst {
		node := s.nodes[v]
		if e = f(ctx, node.name, node.propmt); e != nil {
			return
		}
	}
	return
}

func GetResult(ctx context.Context) (r map[string]string) {
	r, _ = ctx.Value("graph-result").(map[string]string)
	return
}

func SetResult(ctx context.Context, r map[string]string) context.Context {
	return context.WithValue(ctx, "graph-result", r)
}

func SetResultKV(ctx context.Context, k, v string) context.Context {
	md := GetResult(ctx)
	md[k] = v
	return SetResult(ctx, md)
}
