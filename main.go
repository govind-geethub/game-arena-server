package main

import "fmt"

func place(g *Game, id string, x, y int) {
	g.Players[id] = &Player{ID: id, X: x, Y: y}
}

func main() {
	g := NewGame(12)

	// Test 1: charged double attack, Al shoots both neighbours
	place(g, "Al", 5, 5)
	g.Players["Al"].Charge = MaxCharge
	place(g, "Sam", 4, 5)
	place(g, "Mia", 6, 5)

	fb := g.ResolveRound(map[string]Action{
		"Al": {Type: ActionDouble, Dir: Left, Dir2: Right},
	})

	fmt.Println("Double Attack: Al score", float64(g.Players["Al"].Score)/UnitsPerKill, "Charge", g.Players["Al"].Charge)
	fmt.Println("Sam shot from:", fb["Sam"].ShotFrom, "Mia shot from:", fb["Mia"].ShotFrom)

	// Test 2: two shooters hit the same victim, so kill is shared
	place(g, "Al", 5, 5)
	place(g, "Sam", 4, 5)
	place(g, "Mia", 6, 5)
	g.ResolveRound(map[string]Action{
		"Sam": {Type: ActionShoot, Dir: Right},
		"Mia": {Type: ActionShoot, Dir: Left},
	})
	fmt.Println("Shared kill: Sam", float64(g.Players["Sam"].Score)/UnitsPerKill, "Mia", float64(g.Players["Mia"].Score)/UnitsPerKill)

	// Test 3: sector alerts
	place(g, "Al", 5, 5)
	place(g, "Sam", 5, 4)
	place(g, "Mia", 4, 5)
	place(g, "Raj", 5, 7)
	v, _ := g.ViewFor("Al")
	fmt.Println("Alerts: Front", v.Front, "Back", v.Back, "Sides", v.Sides, "Near", v.Near)
}
