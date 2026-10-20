package main

import (
	"context"
	"time"
)

type Input struct {
	PlayerID string
	Action   Action
}

type Server struct {
	game     *Game
	inputs   chan Input
	roundLen time.Duration
	round    int

	// OnRound called after every round ends
	OnRound func(rount int, fb map[string]*Feedback)
}

func NewServer(game *Game, roundLen time.Duration) *Server {
	return &Server{
		game:     game,
		inputs:   make(chan Input, 256),
		roundLen: roundLen,
	}
}

// Submit ignores the request if the channel is full
func (s *Server) Submit(in Input) {
	select {
	case s.inputs <- in:
	default:
	}
}

// drain removes the old and keeps only the latest request of every user
func (s *Server) drain() map[string]Action {
	latest := make(map[string]Action)
	for {
		select {
		case in := <-s.inputs:
			latest[in.PlayerID] = in.Action
		default:
			return latest
		}
	}
}

// Run plays match: one round per roundLen
func (s *Server) Run(ctx context.Context, MaxRounds int) {
	ticker := time.NewTicker(s.roundLen)
	defer ticker.Stop()

	for s.round < MaxRounds {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.round++
			fb := s.game.ResolveRound(s.drain())
			if s.OnRound != nil {
				s.OnRound(s.round, fb)
			}
		}
	}
}
