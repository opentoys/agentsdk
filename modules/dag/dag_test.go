package dag

import (
	"context"
	"fmt"
	"log"
	"testing"
)

func TestXxx(t *testing.T) {
	var g = New()
	g.AddNode("A", "asjdhaskd 1")
	g.AddNode("B", "asjdhaskd2")
	g.AddNode("C", "asjdhaskd3")
	g.AddNode("D", "asjdhaskd4")
	g.AddNode("E", "asjdhaskd5")
	g.AddNode("F", "asjdhaskd F")

	g.AddEdge("A", "B")
	g.AddEdge("A", "E")
	g.AddEdge("B", "D")
	g.AddEdge("C", "E")
	g.AddEdge("E", "F")

	// fmt.Println(g)
	e := g.Run(context.Background(), func(ctx context.Context, id, propmt string) (e error) {
		fmt.Println(id, propmt)
		SetResultKV(ctx, id, propmt)
		fmt.Println("-------------------")
		fmt.Println(GetResult(ctx))
		fmt.Println("===================")
		return
	})
	if e != nil {
		log.Fatal(e)
	}
}
