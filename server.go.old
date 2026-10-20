package main

import (
	"context"
	"log"
	"time"
)

type Input struct {
	PlayerID string
	Dir      Direction
}

type Server struct {
	game   *Game
	inputs chan Input
	tick   time.Duration
	count  int
}

func NewServer(size int, ticksPerSecond int) *Server {
	return &Server{
		game:   NewGame(size),
		inputs: make(chan Input, 256),
		tick:   time.Second / time.Duration(ticksPerSecond),
	}
}

// how dynamically input is sent
// if buffer full input is dropped
func (s *Server) Submit(in Input) {
	select {
	case s.inputs <- in:
	default:
	}
}

// Game loop, gets blocked until ctx is cancelled
func (s *Server) Run(ctx context.Context) {
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.step()
		}
	}
}

// drain collects everything in the channeel
// latest input of the player will be considered
func (s *Server) drain() map[string]Direction {
	latest := make(map[string]Direction)
	for {
		select {
		case in := <-s.inputs:
			latest[in.PlayerID] = in.Dir
		default:
			return latest
		}
	}
}

// step runs one tick: apply each player's latest input once
func (s *Server) step() {
	s.count++
	for id, dir := range s.drain() {
		if s.game.Move(id, dir) {
			log.Printf("Tick %d: %s scored", s.count, id)
		}
	}
}
