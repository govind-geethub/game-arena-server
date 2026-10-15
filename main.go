package main

import "fmt"

func main() {
	g := NewGame(10)
	g.AddPlayer("Al")
	g.AddPlayer("Sam")

	for _, p := range g.Players {
		fmt.Println("Before: ", p.ID, p.X, p.Y)
	}

	g.Move("Al", Left)
	for _, p := range g.Players {
		fmt.Println("After: ", p.ID, p.X, p.Y)
	}
}
