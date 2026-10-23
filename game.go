package main

import (
	"math"
	"math/rand"
)

const (
	UnitsPerKill   = 840
	MaxCharge      = 3
	CellsPerPlayer = 10
	NumDirections  = 8
)

// Chunk 1: Types and Directions
type Direction int

const (
	Up Direction = iota
	Down
	Left
	Right
	UpLeft
	UpRight
	DownLeft
	DownRight
)

func (d Direction) Valid() bool { return d >= 0 && d < NumDirections }

func (d Direction) String() string {
	names := [...]string{"Up", "Down", "Left", "Right", "Up-Left", "Up-Right", "Down-Left", "Down-Right"}
	if d.Valid() {
		return names[d]
	}
	return "Unknown"
}

func delta(d Direction) (int, int) {
	switch d {
	case Up:
		return 0, -1
	case Down:
		return 0, 1
	case Left:
		return -1, 0
	case Right:
		return 1, 0
	case UpLeft:
		return -1, -1
	case UpRight:
		return 1, -1
	case DownLeft:
		return -1, 1
	case DownRight:
		return 1, 1
	}
	return 0, 0
}

func dirOf(dx, dy int) Direction {
	for d := Direction(0); d < NumDirections; d++ {
		ddx, ddy := delta(d)
		if ddx == dx && ddy == dy {
			return d
		}
	}
	return Up
}

type Sector int

const (
	SectorFront Sector = iota
	SectorBack
	SectorSides
)

func sectorOf(d Direction) Sector {
	switch d {
	case Up, UpLeft, UpRight:
		return SectorFront
	case Down, DownLeft, DownRight:
		return SectorBack
	}
	return SectorSides
}

type ActionType int

const (
	ActionSkip ActionType = iota // evolves to Scan
	ActionMove
	ActionShoot
	ActionDouble
)

type Action struct {
	Type ActionType
	Dir  Direction
	Dir2 Direction // only for Double Attack
}

type Player struct {
	ID     string
	X, Y   int
	Score  int
	Charge int
}

type Game struct {
	Size    int
	Players map[string]*Player
}

// what happend to player's shot
type ShotResult struct {
	Dir Direction
	Hit bool
}

// reponse after every round
type Feedback struct {
	Shots    []ShotResult // shot results
	ShotFrom []Direction  // direction from where you were hit
	Died     bool
	DidScan  bool
	Scan     []Direction // directions of adjacent enemies
	Notice   string      // why the request was refused, if it was
}

type View struct {
	X, Y   int
	Charge int
	Front  int // up, up-left, up-right
	Back   int // down, down-left, down-right
	Sides  int // left, right
	Near   int // enemies at distance 2
}

// Chunk 2: The Board
func NewGame(size int) *Game {
	return &Game{Size: size, Players: make(map[string]*Player)}
}

func GridSizeFor(players int) int {
	size := int(math.Ceil(math.Sqrt(float64(players * CellsPerPlayer))))
	if size < 6 {
		size = 6
	}
	return size
}

func abs(n int) int {
	if n < 0 {
		return -n
	}

	return n
}

// dist is the larger of the horizontal and vertical gap
func distance(ax, ay, bx, by int) int {
	dx, dy := abs(ax-bx), abs(ay-by)
	if dx > dy {
		return dx
	}
	return dy
}

func (g *Game) inBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < g.Size && y < g.Size
}

func (g *Game) CellTaken(x, y int) *Player {
	for _, p := range g.Players {
		if p.X == x && p.Y == y {
			return p
		}
	}
	return nil
}

// counts players other than id within distance r of (x, y)
func (g *Game) enemiesWithin(id string, x, y, r int) int {
	n := 0
	for _, p := range g.Players {
		if p.ID != id && distance(x, y, p.X, p.Y) <= r {
			n++
		}
	}
	return n
}

func (g *Game) adjacentDirs(id string) []Direction {
	p, ok := g.Players[id]
	if !ok {
		return nil
	}

	var dirs []Direction
	for d := Direction(0); d < NumDirections; d++ {
		dx, dy := delta(d)
		if g.CellTaken(p.X+dx, p.Y+dy) != nil {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// finds empty cell with no enemy within radius 2 units
// if not then a random cell
func (g *Game) pickSpawn(id string) (int, int, bool) {
	for i := 0; i < 200; i++ {
		x, y := rand.Intn(g.Size), rand.Intn(g.Size)
		if g.CellTaken(x, y) == nil && g.enemiesWithin(id, x, y, 2) == 0 {
			return x, y, true
		}
	}

	for y := 0; y < g.Size; y++ {
		for x := 0; x < g.Size; x++ {
			if g.CellTaken(x, y) == nil {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

func (g *Game) AddPlayer(id string) bool {
	x, y, ok := g.pickSpawn(id)
	if !ok {
		return false
	}
	g.Players[id] = &Player{ID: id, X: x, Y: y}
	return true
}

func (g *Game) RemovePlayer(id string) {
	delete(g.Players, id)
}

func (g *Game) Respawn(id string) {
	p, ok := g.Players[id]
	if !ok {
		return
	}

	if x, y, found := g.pickSpawn(id); found {
		p.X, p.Y = x, y
	}
}

// Chunk 3: Rules of the Round
// with additional response use / not-use
func (g *Game) normalise(p *Player, a Action) (Action, bool, string) {
	switch a.Type {
	case ActionSkip:
		return a, true, ""
	case ActionMove, ActionShoot:
		if a.Dir.Valid() {
			return a, true, ""
		}
		return a, false, "invalid direction"
	case ActionDouble:
		switch {
		case len(g.Players) <= 2:
			return a, false, "not enough players to activate this option"
		case p.Charge < MaxCharge:
			return a, false, "double attack is not charged yet"
		case !a.Dir.Valid() || !a.Dir2.Valid() || a.Dir == a.Dir2:
			return a, false, "pick two different directions"
		}
		return a, true, ""
	}
	return a, false, "unknown action"
}

// all actions apllied at once
// feedback: each player learns. players not on map did nothing
func (g *Game) ResolveRound(actions map[string]Action) map[string]*Feedback {
	type pos struct{ x, y int }

	fb := make(map[string]*Feedback, len(g.Players))
	start := make(map[string]pos, len(g.Players))
	for id, p := range g.Players {
		fb[id] = &Feedback{}
		start[id] = pos{p.X, p.Y}
	}

	// validate before anything changes
	effective := make(map[string]Action, len(actions))
	for id, a := range actions {
		p, ok := g.Players[id]
		if !ok {
			continue
		}
		valid, allowed, notice := g.normalise(p, a)
		if !allowed {
			fb[id].Notice = notice // nothing happens, and no free scan
			continue
		}
		effective[id] = valid
	}

	// # Phase 1: shots, against positions at the start of the round
	shotsOn := make(map[string][]string) // victim -> shooters
	usedCharge := make(map[string]bool)

	for id, a := range effective {
		p := g.Players[id]
		var dirs []Direction
		switch a.Type {
		case ActionShoot:
			dirs = []Direction{a.Dir}
		case ActionDouble:
			dirs = []Direction{a.Dir, a.Dir2}
			p.Charge = 0
			usedCharge[id] = true
		default:
			continue
		}

		for _, d := range dirs {
			dx, dy := delta(d)
			v := g.CellTaken(p.X+dx, p.Y+dy)
			fb[id].Shots = append(fb[id].Shots, ShotResult{Dir: d, Hit: v != nil})
			if v != nil {
				shotsOn[v.ID] = append(shotsOn[v.ID], id)
			}
		}
	}

	// # Phase 2: scoring (splitting score for whoever killed)
	for vid, shooters := range shotsOn {
		share := UnitsPerKill / len(shooters)
		for _, sid := range shooters {
			g.Players[sid].Score += share
			dx := start[sid].x - start[vid].x
			dy := start[sid].y - start[vid].y
			fb[vid].ShotFrom = append(fb[vid].ShotFrom, dirOf(dx, dy))
		}
		fb[vid].Died = true
	}

	// # Phase 3: moves (players who were shot don't move)
	targets := make(map[string]pos)
	wanted := make(map[pos]int)

	for id, a := range effective {
		if a.Type != ActionMove || fb[id].Died {
			continue
		}

		p := g.Players[id]
		dx, dy := delta(a.Dir)
		nx, ny := p.X+dx, p.Y+dy

		if !g.inBounds(nx, ny) || g.CellTaken(nx, ny) != nil {
			continue
		}

		t := pos{nx, ny}
		targets[id] = t
		wanted[t]++
	}

	for id, t := range targets {
		if wanted[t] == 1 {
			g.Players[id].X, g.Players[id].Y = t.x, t.y
		}
	}

	// #Phase 4: respawn everyone who was shot
	for id, f := range fb {
		if f.Died {
			g.Respawn(id)
		}
	}

	// #Phase 5: charge goes up (or -> 0 if used)
	for id, p := range g.Players {
		if !usedCharge[id] && p.Charge < MaxCharge {
			p.Charge++
		}
	}

	// #Phase 6: skipping scans the neighbours, as they stand now
	for id, a := range effective {
		if a.Type == ActionSkip && !fb[id].Died {
			fb[id].DidScan = true
			fb[id].Scan = g.adjacentDirs(id)
		}
	}
	return fb
}

// what a player is allowed to know
func (g *Game) ViewFor(id string) (View, bool) {
	p, ok := g.Players[id]
	if !ok {
		return View{}, false
	}

	v := View{X: p.X, Y: p.Y, Charge: p.Charge}
	for _, d := range g.adjacentDirs(id) {
		switch sectorOf(d) {
		case SectorFront:
			v.Front++
		case SectorBack:
			v.Back++
		default:
			v.Sides++
		}
	}

	adjacent := v.Front + v.Back + v.Sides
	v.Near = g.enemiesWithin(id, p.X, p.Y, 2) - adjacent
	return v, true
}
