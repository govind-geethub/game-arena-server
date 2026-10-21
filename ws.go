package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// ClientMsg: browser sends to each client: one action per msg
type ClientMsg struct {
	Action string `jason: "action"`
	Dir    string `json: "dir"`
	Dir2   string `json:"dir2"`
}

func parseDir(s string) (Direction, bool) {
	for d := Direction(0); d < NumDirections; d++ {
		if d.String() == s {
			return d, true
		}
	}
	return 0, false
}

// parseAction checks browser msg then into Action
func parseAction(m ClientMsg) (Action, bool) {
	switch m.Action {
	case "skip":
		return Action{Type: ActionSkip}, true
	case "move", "shoot":
		d, ok := parseDir(m.Dir)
		if !ok {
			return Action{}, false
		}
		t := ActionMove
		if m.Action == "shoot" {
			t = ActionShoot
		}
		return Action{Type: t, Dir: d}, true
	case "double":
		d1, ok1 := parseDir(m.Dir)
		d2, ok2 := parseDir(m.Dir2)

		if !ok1 || !ok2 {
			return Action{}, false
		}
		return Action{Type: ActionDouble, Dir: d1, Dir2: d2}, true
	}
	return Action{}, false
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func refuse(ctx context.Context, conn *websocket.Conn, msg string) {
	b, _ := json.Marshal(ErrorMsg{Type: "Error", Message: msg})
	conn.Write(ctx, websocket.MessageText, b)
	conn.Close(websocket.StatusPolicyViolation, "Refused")
}

// HandleWS for on player: /ws?code=XXXXXX&name=Al
func (l *Lobby) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}

	defer conn.CloseNow()
	conn.SetReadLimit(1024) // no browser msg needs more than this

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ip := clientIP(r)
	if l.Blocked(ip) {
		refuse(ctx, conn, "Too Many Wrong Codes, Wait a Minute!")
		return
	}

	code := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("code")))
	room := l.Get(code)
	if room == nil {
		l.RecordFail(ip)
		refuse(ctx, conn, "Party Not Found!")
		return
	}

	c, err := room.Join(r.URL.Query().Get("name"))
	if err != nil {
		refuse(ctx, conn, err.Error())
		return
	}

	defer room.Leave(c)

	// writer: sends this player's queued msg
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.done:
				return
			case b := <-c.send:
				wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
				err := conn.Write(wctx, websocket.MessageText, b)
				wcancel()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()

	// Reader: turns this player's msg -> action
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		var m ClientMsg
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		if a, ok := parseAction(m); ok {
			room.Submit(c.ID, a)
		}
	}
}
