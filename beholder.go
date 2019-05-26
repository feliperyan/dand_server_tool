package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

type player struct {
	name string
	conn *websocket.Conn
}

type PlayerMessage struct {
	Action    string `json:"action"`
	Payload   string `json:"payload"`
	Recipient string `json:"recipient"`
	Sender    string `json:"sender"`
}

type beholder struct {
	players  map[string]*player
	Messages chan PlayerMessage
	joining  chan *player
	leaving  chan *player
	showRose chan os.Signal
	dead     chan struct{}
}

func spawnEvil() *beholder {
	return &beholder{players: make(map[string]*player),
		Messages: make(chan PlayerMessage),
		joining:  make(chan *player),
		leaving:  make(chan *player),
		showRose: make(chan os.Signal, 1),
		dead:     make(chan struct{}, 1)}
}

func (be *beholder) broadcast(msg PlayerMessage, close bool) {
	if close {
		for _, p := range be.players {
			err := p.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write: ", err)
			}
		}
	} else {
		for _, p := range be.players {
			if p.name != msg.Sender { // dont echo it to sender
				err := p.conn.WriteMessage(websocket.TextMessage, getJSONPlayerMessage(msg))
				if err != nil {
					log.Println("write: ", err)
				}
			}

		}
	}
}

func getJSONPlayerMessage(msg PlayerMessage) []byte {
	value, err := json.Marshal(msg)
	if err != nil {
		log.Println("error marshalling: ", err)
	}

	return value
}

func createJSONMessage(command string) []byte {
	action := strings.Trim(command, " ")
	tokens := strings.Split(action, " ")

	msg := &PlayerMessage{}

	switch tokens[0] {
	case "/file":
		msg = &PlayerMessage{Action: "file", Payload: tokens[1], Recipient: ""}
	default:
		msg = &PlayerMessage{Action: "say", Payload: action, Recipient: ""}
	}

	value, err := json.Marshal(msg)
	if err != nil {
		log.Println("error marshalling: ", err)
	}

	return value
}

func (be *beholder) broadcastFile(filePath string) {
	dat, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("Error opening file: %s\n", err)
	}

	for _, p := range be.players {
		err := p.conn.WriteMessage(websocket.BinaryMessage, dat)
		if err != nil {
			log.Println("error sending file: ", err)
		}
	}
}

func (be *beholder) changeName(original string, newName string) {
	fmt.Printf("Set name for: %s as %s\n", original, newName)
	be.players[newName] = be.players[original]
	be.players[newName].name = newName
	delete(be.players, original)
	msg := PlayerMessage{Recipient: newName, Payload: fmt.Sprintf("name changed to: %s", newName), Sender: "Beholder"}

	be.whisper(msg)
}

func (be *beholder) whisper(msg PlayerMessage) {
	p, ok := be.players[msg.Recipient]
	msg.Action = "say"
	if ok {
		log.Printf("P ok: %s is %s\n", p.name, msg.Recipient)
		err := p.conn.WriteMessage(websocket.TextMessage, getJSONPlayerMessage(msg))
		if err != nil {
			log.Println("write: ", err)
		}
	}

}

func (be *beholder) listPlayers(playerName string) {
	allPlayers := make([]string, 0)
	for k := range be.players {
		allPlayers = append(allPlayers, k)
	}
	msg := PlayerMessage{Recipient: playerName, Payload: fmt.Sprintf("Players: %s", strings.Join(allPlayers, "; ")), Sender: "Beholder"}
	be.whisper(msg)
}

func (be *beholder) processMessage(msg PlayerMessage) {
	switch msg.Action {
	case "say":
		be.broadcast(msg, false)
	case "whisper":
		be.whisper(msg)
	case "setname":
		be.changeName(msg.Sender, msg.Payload)
	case "list":
		be.listPlayers(msg.Sender)
	case "file":
		be.broadcastFile(msg.Payload)
	}

}

func (be *beholder) openEye() {
	go func() {
	topFor:
		for {
			select {
			case msg := <-be.Messages:
				log.Printf("Beholder: %s", msg)
				be.processMessage(msg)

			case newPlayer := <-be.joining:
				be.players[newPlayer.name] = newPlayer
			case leavingPlayer := <-be.leaving:
				delete(be.players, leavingPlayer.name)
			case <-be.showRose: // kill the server
				be.broadcast(PlayerMessage{}, true) // tell all clients to disconnect
				break topFor
			}
		}
		close(be.dead)
		return
	}()
}
