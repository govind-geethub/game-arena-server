package main

import "fmt"

func main() {
	g := NewGame(10)
	g.AddPlayer("Al")
	g.AddPlayer("Sam")

	g.Players["Al"].X, g.Players["Al"].Y = 0, 0
	g.Players["Sam"].X, g.Players["Sam"].Y = 1, 0

	hit := g.Move("Al", Right)
	fmt.Println("Hit: ", hit)

	for _, p := range g.Players {
		fmt.Println(p.ID, "pos: ", p.X, p.Y, "Score: ", p.Score)
	}

	// shooting the boundary does nothing
	hit = g.Move("Al", Left)
	fmt.Println("Hit: ", hit, "Al: ", g.Players["Al"].X, g.Players["Al"].Y)
}
