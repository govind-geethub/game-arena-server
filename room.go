package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"sync"
	"time"
)

const (
	MaxPlayers    = 8
	idleTimeout   = 30 * time.Minute
	maxSpectators = 4
)

var (
	nameRe       = regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`)
	ErrForbidden = errors.New("forbidden")
)

// ----- msg sent to Browsers -----
type LobbyMsg struct {
	Type    string   `json:"type"`
	Code    string   `json:"code"`
	Players []string `json:"players"`
}

type StartMsg struct {
	Type     string `json:"type"`
	GridSize int    `json:"gridSize"`
	Rounds   int    `json:"rounds"`
	RoundMs  int    `json:"roundMs"`
	Players  int    `json:"players"`
}

type ShotMsg struct {
	Dir string `json:"dir"`
	Hit bool   `json:"hit"`
}

// RoundMsg is one player's private info
// ViewFor + Feedback
type RoundMsg struct {
	Type     string    `json:"type"`
	Round    int       `json:"round"`
	X        int       `json:"x"`
	Y        int       `json:"y"`
	Charge   int       `json:"charge"`
	Front    int       `json:"front"`
	Back     int       `json:"back"`
	Sides    int       `json:"sides"`
	Near     int       `json:"near"`
	Score    float64   `json:"score"`
	Shots    []ShotMsg `json:"shots"`
	ShotFrom []string  `json:"shotFrom"`
	Died     bool      `json:"died"`
	DidScan  bool      `json:"didScan"`
	Scan     []string  `json:"scan"`
	Notice   string    `json:"notice"`
}

type ScoreMsg struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

type EndMsg struct {
	Type   string     `json:"type"`
	Scores []ScoreMsg `json:"scores"`
}

type ErrorMsg struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type GodPlayer struct {
	Name    string    `json:"name"`
	X       int       `json:"x"`
	Y       int       `json:"y"`
	Charge  int       `json:"charge"`
	Score   float64   `json:"score"`
	Died    bool      `json:"died"`
	DidScan bool      `json:"didScan"`
	Shots   []ShotMsg `json:"shots"`
}

type GodMsg struct {
	Type     string      `json:"type"`
	Round    int         `json:"round"`
	GridSize int         `json:"gridSize"`
	Rounds   int         `json:"rounds"`
	Players  []GodPlayer `json:"players"`
}

// ---- client ----

type Client struct {
	ID   string
	send chan []byte
	done chan struct{}
	once sync.Once
}

func newClient(id string) *Client {
	return &Client{ID: id, send: make(chan []byte, 16), done: make(chan struct{})}
}

// Send never blocks, if client is too slow its msg is skipped
func (c *Client) Send(msg any) {
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}

	select {
	case c.send <- b:
	default:
	}
}

func (c *Client) Close() { c.once.Do(func() { close(c.done) }) }

// ---- room ----
type Room struct {
	code      string
	hostToken string
	roundLen  time.Duration
	maxRounds int
	createdAt time.Time
	onDone    func()

	mu         sync.Mutex
	clients    map[string]*Client
	started    bool
	finished   bool
	game       *Game
	server     *Server
	spectators map[*Client]bool
	curRound   int
}

func NewRoom(code, hostToken string, roundLen time.Duration, maxRounds int) *Room {
	return &Room{
		code:       code,
		hostToken:  hostToken,
		roundLen:   roundLen,
		maxRounds:  maxRounds,
		createdAt:  time.Now(),
		clients:    make(map[string]*Client),
		spectators: make(map[*Client]bool),
	}
}

func (r *Room) lobbyLocked() LobbyMsg {
	names := make([]string, 0, len(r.clients))
	for name := range r.clients {
		names = append(names, name)
	}

	sort.Strings(names)
	return LobbyMsg{Type: "lobby", Code: r.code, Players: names}
}

func (r *Room) broadcastLocked(msg any) {
	for _, c := range r.clients {
		c.Send(msg)
	}

	for c := range r.spectators {
		c.Send(msg)
	}
}

// Join adds a player only can join if match has not start
func (r *Room) Join(name string) (*Client, error) {
	if !nameRe.MatchString(name) {
		return nil, errors.New("Name must be 1-16 letter, digits or underscores")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return nil, errors.New("Match already started")
	}

	if len(r.clients) >= MaxPlayers {
		return nil, errors.New("Party is Full")
	}

	if _, taken := r.clients[name]; taken {
		return nil, errors.New("Name already taken")
	}

	c := newClient(name)
	r.clients[name] = c
	r.broadcastLocked(r.lobbyLocked())
	return c, nil
}

// Leave removes the player from the party
func (r *Room) Leave(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return
	}

	if r.clients[c.ID] == c {
		delete(r.clients, c.ID)
		r.broadcastLocked(r.lobbyLocked())
	}
}

// Submit passes players' action to match
func (r *Room) Submit(id string, a Action) {
	r.mu.Lock()
	s, done := r.server, r.finished
	r.mu.Unlock()

	if s != nil && !done {
		s.Submit(Input{PlayerID: id, Action: a})
	}
}

// sendRondLocked builds one player's own update
func (r *Room) sendRoundLocked(round int, id string, c *Client, f *Feedback) {
	v, ok := r.game.ViewFor(id)
	if !ok {
		return
	}

	if f == nil {
		f = &Feedback{}
	}
	shots := make([]ShotMsg, 0, len(f.Shots))
	for _, s := range f.Shots {
		shots = append(shots, ShotMsg{Dir: s.Dir.String(), Hit: s.Hit})
	}
	c.Send(RoundMsg{
		Type: "round", Round: round,
		X: v.X, Y: v.Y, Charge: v.Charge,
		Front: v.Front, Back: v.Back, Sides: v.Sides, Near: v.Near,
		Score:    float64(r.game.Players[id].Score) / UnitsPerKill,
		Shots:    shots,
		ShotFrom: dirNames(f.ShotFrom),
		Died:     f.Died,
		DidScan:  f.DidScan,
		Scan:     dirNames(f.Scan),
		Notice:   f.Notice,
	})
}

func dirNames(dirs []Direction) []string {
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		out = append(out, d.String())
	}
	return out
}

// Start runs the match done only by host
func (r *Room) Start(token string) error {
	if subtle.ConstantTimeCompare([]byte(token), []byte(r.hostToken)) != 1 {
		return ErrForbidden
	}

	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return errors.New("Match already started")
	}

	if len(r.clients) < 2 {
		r.mu.Unlock()
		return errors.New("Need atleast 2 players")
	}

	game := NewGame(GridSizeFor(len(r.clients)))
	for id := range r.clients {
		game.AddPlayer(id)
	}
	srv := NewServer(game, r.roundLen)
	srv.OnRound = r.onRound
	r.game, r.server, r.started = game, srv, true

	r.broadcastLocked(StartMsg{
		Type:     "start",
		GridSize: game.Size,
		Rounds:   r.maxRounds,
		RoundMs:  int(r.roundLen / time.Millisecond),
		Players:  len(game.Players),
	})

	// round 0 everyone gets their own position
	for id, c := range r.clients {
		r.sendRoundLocked(0, id, c, nil)
	}
	r.sendGodLocked(0, nil)
	r.mu.Unlock()

	go func() {
		srv.Run(context.Background(), r.maxRounds)
		r.finish()
	}()
	return nil
}

// onRound runs on the match loop after every round
func (r *Room) onRound(round int, fb map[string]*Feedback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.curRound = round
	for id, c := range r.clients {
		r.sendRoundLocked(round, id, c, fb[id])
	}
	r.sendGodLocked(round, fb)
}

// finish gives out the final scores, highest first, then cleans up
func (r *Room) finish() {
	r.mu.Lock()
	scores := make([]ScoreMsg, 0, len(r.game.Players))
	for id, p := range r.game.Players {
		scores = append(scores, ScoreMsg{Name: id, Score: float64(p.Score) / UnitsPerKill})
	}

	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Score != scores[j].Score {
			return scores[i].Score > scores[j].Score
		}
		return scores[i].Name < scores[j].Name
	})

	r.finished = true
	r.broadcastLocked(EndMsg{Type: "end", Scores: scores})
	r.mu.Unlock()

	if r.onDone != nil {
		r.onDone()
	}
	time.AfterFunc(10*time.Second, r.Shutdown)
}

// idleExpired reports whether a party was never startetd and is too old
func (r *Room) idleExpired(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.started && now.Sub(r.createdAt) > idleTimeout
}

func (r *Room) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.clients {
		c.Close()
	}
	for c := range r.spectators {
		c.Close()
	}
}

//-------- God's Eye --------

// every player position
// only to spectators
func (r *Room) godSnapshotLocked(round int, fb map[string]*Feedback) GodMsg {
	msg := GodMsg{Type: "god", Round: round, GridSize: r.game.Size, Rounds: r.maxRounds}
	names := make([]string, 0, len(r.game.Players))
	for id := range r.game.Players {
		names = append(names, id)
	}
	sort.Strings(names)
	for _, id := range names {
		p := r.game.Players[id]
		gp := GodPlayer{
			Name: id, X: p.X, Y: p.Y, Charge: p.Charge,
			Score: float64(p.Score) / UnitsPerKill,
			Shots: []ShotMsg{},
		}

		if f := fb[id]; f != nil {
			gp.Died, gp.DidScan = f.Died, f.DidScan
			for _, s := range f.Shots {
				gp.Shots = append(gp.Shots, ShotMsg{Dir: s.Dir.String(), Hit: s.Hit})
			}
		}
		msg.Players = append(msg.Players, gp)
	}
	return msg
}

func (r *Room) sendGodLocked(round int, fb map[string]*Feedback) {
	if len(r.spectators) == 0 {
		return
	}

	snap := r.godSnapshotLocked(round, fb)
	for c := range r.spectators {
		c.Send(snap)
	}
}

// Spectate adds a read-only viewer
func (r *Room) Spectate(token string) (*Client, error) {
	if subtle.ConstantTimeCompare([]byte(token), []byte(r.hostToken)) != 1 {
		return nil, ErrForbidden
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.spectators) >= maxSpectators {
		return nil, errors.New("Too many spectators")
	}

	c := newClient("spectator")
	r.spectators[c] = true
	c.Send(r.lobbyLocked())
	if r.started {
		c.Send(StartMsg{
			Type: "start", GridSize: r.game.Size, Rounds: r.maxRounds,
			RoundMs: int(r.roundLen / time.Millisecond), Players: len(r.game.Players),
		})
		c.Send(r.godSnapshotLocked(r.curRound, nil))
	}
	return c, nil
}

func (r *Room) DropSpectator(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.spectators, c)
}
