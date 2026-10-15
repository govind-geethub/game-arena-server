package main

import "math/rand"

type Direction int

const (
	Up Direction = iota
	Down
	Left
	Right
)

type Player struct {
	ID    string
	X, Y  int
	Score int
}

type Game struct {
	Size    int
	Players map[string]*Player
}

func NewGame(size int) *Game {
	return &Game{Size: size, Players: make(map[string]*Player)}
}

func (g *Game) CellTaken(x, y int) *Player {
	for _, p := range g.Players {
		if p.X == x && p.Y == y {
			return p
		}
	}
	return nil
}

func (g *Game) RandomEmptyCell() (int, int) {
	for {
		x, y := rand.Intn(g.Size), rand.Intn(g.Size)
		if g.CellTaken(x, y) == nil {
			return x, y
		}
	}
}

func (g *Game) AddPlayer(id string) {
	x, y := g.RandomEmptyCell()
	g.Players[id] = &Player{ID: id, X: x, Y: y}
}

func (g *Game) RemovePlayer(id string) {
	delete(g.Players, id)
}

func (g *Game) Move(id string, d Direction) {
	p, ok := g.Players[id]
	if !ok {
		return
	}

	nx, ny := p.X, p.Y
	switch d {
	case Up:
		ny--
	case Down:
		ny++
	case Left:
		nx--
	case Right:
		nx++
	}

	if nx < 0 || ny < 0 || nx >= g.Size || ny >= g.Size {
		return // outside the boundary
	}

	if g.CellTaken(nx, ny) != nil {
		return // place is taken so shoot or move other position
	}

	p.X, p.Y = nx, ny
}
