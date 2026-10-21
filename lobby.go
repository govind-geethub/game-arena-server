package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	maxRooms          = 50
	maxFailsPerMinute = 10
	codeAlphabet      = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567890"
)

type failRecord struct {
	count int
	since time.Time
}

type Lobby struct {
	mu        sync.Mutex
	rooms     map[string]*Room
	fails     map[string]*failRecord // wrong code attempts per IP addr.
	roundLen  time.Duration
	maxRounds int
}

func NewLobby(roundLen time.Duration, maxRounds int) *Lobby {
	l := &Lobby{
		rooms:     make(map[string]*Room),
		fails:     make(map[string]*failRecord),
		roundLen:  roundLen,
		maxRounds: maxRounds,
	}

	go l.janitor()
	return l
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func newCode() string {
	b := randBytes(6)
	for i := range b {
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b)
}

// Create make s new party and return its code and host token
func (l *Lobby) Create() (string, string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.rooms) >= maxRooms {
		return "", "", errors.New("Server is full, try again later")
	}

	var code string
	for {
		code = newCode()
		if _, taken := l.rooms[code]; !taken {
			break
		}
	}

	token := hex.EncodeToString(randBytes(16))
	room := NewRoom(code, token, l.roundLen, l.maxRounds)
	room.onDone = func() { l.Remove(code) }
	l.rooms[code] = room
	return code, token, nil
}

func (l *Lobby) Get(code string) *Room {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rooms[code]
}

func (l *Lobby) Remove(code string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.rooms, code)
}

// Blocked reports whether an IP has made too many wrong guesses
func (l *Lobby) Blocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	rec, ok := l.fails[ip]
	if !ok {
		return false
	}

	if time.Since(rec.since) > time.Minute {
		delete(l.fails, ip)
		return false
	}
	return rec.count >= maxFailsPerMinute
}

func (l *Lobby) RecordFail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	rec, ok := l.fails[ip]
	if !ok || time.Since(rec.since) > time.Minute {
		l.fails[ip] = &failRecord{count: 1, since: time.Now()}
	}
	rec.count++
}

// janitor removes parties which were never started
func (l *Lobby) janitor() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		var expired []*Room
		l.mu.Lock()

		for code, room := range l.rooms {
			if room.idleExpired(now) {
				delete(l.rooms, code)
				expired = append(expired, room)
			}
		}

		for ip, rec := range l.fails {
			if now.Sub(rec.since) > time.Minute {
				delete(l.fails, ip)
			}
		}

		l.mu.Unlock()
		for _, room := range expired {
			room.ShutDown()
		}
	}
}

// HandleCreate: POST /create
func (l *Lobby) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Use POST", http.StatusMethodNotAllowed)
		return
	}

	code, token, err := l.Create()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"code": code, "token": token})
}

// HandleStart: POST/start?code=XXXXXX with header X-Host-Token
func (l *Lobby) HandleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}

	code := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("code")))
	room := l.Get(code)
	if room == nil {
		http.Error(w, "Party Not Found", http.StatusNotFound)
		return
	}

	if err := room.Start(r.Header.Get("X-Host-Token")); err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, "Forbidden", http.StatusForbidden)
		} else {
			http.Error(w, err.Error(), http.StatusConflict)
		}
		return
	}
	w.Write([]byte("Match Started\n"))
}
