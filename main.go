package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

func place(g *Game, id string, x, y int) {
	g.Players[id] = &Player{ID: id, X: x, Y: y}
}

func demo() {
	g := NewGame(12)

	// test 1: double attack
	place(g, "Al", 5, 5)
	g.Players["Al"].Charge = MaxCharge
	place(g, "Sam", 4, 5)
	place(g, "Mia", 6, 5)

	fb := g.ResolveRound(map[string]Action{
		"Al": {Type: ActionDouble, Dir: Left, Dir2: Right},
	})

	fmt.Println("Double Attack: Al score", float64(g.Players["Al"].Score)/UnitsPerKill, "charge", g.Players["Al"].Charge)
	fmt.Println("Sam shot from:", fb["Sam"].ShotFrom, "Mia shot from", fb["Mia"].ShotFrom)

	// Test 2: shared kill
	place(g, "Al", 5, 5)
	place(g, "Sam", 4, 5)
	place(g, "Mia", 6, 5)
	g.ResolveRound(map[string]Action{
		"Sam": {Type: ActionShoot, Dir: Right},
		"Mia": {Type: ActionShoot, Dir: Left},
	})
	fmt.Println("Shared Kill: Sam", float64(g.Players["Sam"].Score)/UnitsPerKill)

	// Test 3: Sector alerts
	place(g, "Al", 5, 5)
	place(g, "Same", 5, 4)
	place(g, "Mia", 4, 5)
	place(g, "Raj", 5, 7)

	v, _ := g.ViewFor("Al")
	fmt.Println("Alerts: Front", v.Front, "Back", v.Back, "Sides", v.Sides, "Near", v.Near)
}

// bot sends inputs
// channel just reads it so no race condition around diff. goroutines
func bot(ctx context.Context, s *Server, id string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(50+rand.Intn(150)) * time.Millisecond):
			s.Submit(Input{PlayerID: id, Action: Action{
				Type: ActionType(rand.Intn(4)),
				Dir:  Direction(rand.Intn(NumDirections)),
				Dir2: Direction(rand.Intn(NumDirections)),
			}})
		}
	}
}

func main() {
	demo()
	fmt.Println("---- Bot Match ----")

	ids := []string{"Al", "Sam", "Mia", "Raj", "Sia"}
	game := NewGame(5)

	for _, id := range ids {
		game.AddPlayer(id)
	}

	s := NewServer(game, 200*time.Millisecond)
	s.OnRound = func(round int, fb map[string]*Feedback) {
		for id, f := range fb {
			for _, shot := range f.Shots {
				if shot.Hit {
					fmt.Printf("Round %d: %s hit someone (%v)\n", round, id, shot.Dir)
				}
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	for _, id := range ids {
		go bot(ctx, s, id)
	}

	s.Run(ctx, 60)
	cancel()

	fmt.Println("Final Scores")
	for _, id := range ids {
		p := game.Players[id]
		fmt.Printf("%s, %.2f points\n", id, float64(p.Score)/UnitsPerKill)
	}
}
