package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"whispssh-server/types"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	channels = make(map[string]*types.Channel)
	mutex    sync.Mutex
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func CreateChannel(w http.ResponseWriter, r *http.Request) {
	var req types.CreateChannelRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid request"})
		return
	}

	channelID := uuid.New().String()
	channel := &types.Channel{
		ID:       channelID,
		Password: req.Password,
		Clients:  make(map[string]*websocket.Conn),
	}

	mutex.Lock()
	channels[channelID] = channel
	mutex.Unlock()

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, types.CreateChannelResponse{ChannelID: channelID})
}

func JoinChannel(w http.ResponseWriter, r *http.Request) {
	channelID := chi.URLParam(r, "channelID")
	password := r.URL.Query().Get("password")

	mutex.Lock()
	channel, exists := channels[channelID]
	mutex.Unlock()

	if !exists || channel.Password != password {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{"error": "Invalid channel ID or password"})
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade: %v", err)
		return
	}

	clientID := uuid.New().String()

	channel.Mutex.Lock()
	channel.Clients[clientID] = conn
	channel.Mutex.Unlock()

	go handleMessages(conn, channel, clientID)
}

func handleMessages(conn *websocket.Conn, channel *types.Channel, clientID string) {
	defer func() {
		conn.Close()
		channel.Mutex.Lock()
		delete(channel.Clients, clientID)
		channel.Mutex.Unlock()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		channel.Mutex.Lock()
		for id, client := range channel.Clients {
			if id != clientID {
				if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
					fmt.Printf("Error sending to client %s: %v\n", id, err)
				}
			}
		}
		channel.Mutex.Unlock()
	}
}
