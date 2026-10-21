package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "demo" {
		runDemo()
		return
	}

	lobby := NewLobby(3*time.Second, 20)

	mux := http.NewServeMux()
	mux.HandleFunc("/create", lobby.HandleCreate)
	mux.HandleFunc("/ws", lobby.HandleWS)
	mux.HandleFunc("/start", lobby.HandleStart)
	mux.Handle("/", http.FileServer(http.Dir("static")))

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second, // drops the client who connect & stall
	}

	log.Println("Listening on :8080")
	log.Fatal(srv.ListenAndServe())
}
