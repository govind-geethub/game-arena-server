package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

func bot(ctx context.Context, s *Server, id string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(50+rand.Intn(150)) * time.Millisecond):
			s.Submit(Input{PlayerID: id, Dir: Direction(rand.Intn(4))})
		}
	}
}

func main() {
	s := NewServer(3, 2) // 5x5 grid, 10 ticks per second

	ids := []string{"Al", "Sam", "Mia", "Raj"}
	for _, id := range ids {
		s.game.AddPlayer(id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	for _, id := range ids {
		go bot(ctx, s, id)
	}

	s.Run(ctx) // block for 5 sec
	fmt.Println("Final Scores: ")
	for _, p := range s.game.Players {
		fmt.Println(p.ID, p.Score)
	}
}
