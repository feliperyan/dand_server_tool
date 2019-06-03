package main

import (
	"encoding/json"
	"fmt"
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
	players        map[string]*player
	Messages       chan PlayerMessage
	joining        chan *player
	leaving        chan *player
	showRose       chan os.Signal
	dead           chan struct{}
	audioChan      chan []byte
	whoToSendAudio string
}

func spawnEvil() *beholder {
	return &beholder{players: make(map[string]*player),
		Messages:       make(chan PlayerMessage),
		joining:        make(chan *player),
		leaving:        make(chan *player),
		showRose:       make(chan os.Signal, 1),
		dead:           make(chan struct{}, 1),
		audioChan:      make(chan []byte),
		whoToSendAudio: ""}
}

func (be *beholder) broadcast(msg PlayerMessage, close bool) {
	if close { // using this method to close all connections as well as broadcasting messages
		for _, p := range be.players {
			err := p.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write: ", err)
			}
		}
	} else {
		for _, p := range be.players {
			if p.name != msg.Sender { // dont echo it to sender
				newMsg := msg
				newMsg.Recipient = p.name
				fmt.Println("whispering:", newMsg)
				be.whisper(newMsg)
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

func (be *beholder) broadcastAudio(audioFile []byte) {

	for _, p := range be.players {
		// if sending to specific player
		if be.whoToSendAudio != "" {
			if p.name == be.whoToSendAudio {
				err := p.conn.WriteMessage(websocket.BinaryMessage, audioFile)
				if err != nil {
					log.Println("error sending file: ", err)
				}
				be.whoToSendAudio = ""
				return
			}
		}
		err := p.conn.WriteMessage(websocket.BinaryMessage, audioFile)
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

			case sounds := <-be.audioChan:
				be.broadcastAudio(sounds)
			}
		}
		close(be.dead)
		return
	}()
}
