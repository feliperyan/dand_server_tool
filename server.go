package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var oneEyedOne *beholder
var addr = flag.String("addr", "localhost:8080", "http service address")
var upgrader = websocket.Upgrader{} // use default options

// http.HandleFunc will spin up a new goroutine for this
func receive(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	newPlayerName := uuid.New()
	thisPlayer := player{name: newPlayerName.String(), conn: c}
	oneEyedOne.joining <- &thisPlayer

	for {
		mt, Message, err := c.ReadMessage()
		if err != nil {
			log.Println("Error: ", err)
			break
		}

		if mt == websocket.TextMessage {
			msg := &PlayerMessage{}
			if err := json.Unmarshal(Message, &msg); err != nil {
				log.Println("error unmarshaling: ", err)
				break
			}
			msg.Sender = thisPlayer.name
			oneEyedOne.Messages <- *msg
		} else {
			log.Printf("Received a non text Message of type %d", mt)
		}
	}
	oneEyedOne.leaving <- &thisPlayer
}

func monitorExit(allSeeing *beholder, notifyClose chan struct{}, srv *http.Server) {
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	sig := <-sigint // going to block here till we get a value on this channel

	fmt.Println("\nSlaying beholder")
	allSeeing.showRose <- sig

	<-allSeeing.dead // wait for beholder to kill all cons
	fmt.Println("Server shutting down...")

	if err := srv.Shutdown(context.Background()); err != nil {
		// Error from closing listeners, or context timeout:
		log.Printf("HTTP server Shutdown Error: %v", err)
	}
	close(notifyClose)
}

func main() {
	fmt.Println("Starting...")

	flag.Parse()
	log.SetFlags(0)

	servantOfOneEyedOne := &http.Server{Addr: *addr, Handler: nil}
	oneEyedOne = spawnEvil()
	oneEyedOne.openEye()

	connsClosed := make(chan struct{})
	go monitorExit(oneEyedOne, connsClosed, servantOfOneEyedOne)

	http.HandleFunc("/receive", receive)
	log.Fatal(servantOfOneEyedOne.ListenAndServe())

	<-connsClosed
	fmt.Println("back in main")

}
