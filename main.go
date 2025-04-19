package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Simple structures for channel and message
type Channel struct {
	ID       string
	Password string
	Clients  map[string]*websocket.Conn
	Mutex    sync.Mutex
}

// Global state - simple map of channels
var (
	channels = make(map[string]*Channel)
	mutex    sync.Mutex
)

// Request/response structures
type CreateChannelRequest struct {
	Password string `json:"password"`
}

type CreateChannelResponse struct {
	ChannelID string `json:"channel_id"`
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections for simplicity
	},
}

func main() {
	// Define routes
	http.HandleFunc("/channel", createChannel)
	http.HandleFunc("/channel/", joinChannel)

	// Start server
	addr := "127.0.0.1:3000"
	fmt.Printf("Server running on http://%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func createChannel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Create channel
	channelID := uuid.New().String()
	channel := &Channel{
		ID:       channelID,
		Password: req.Password,
		Clients:  make(map[string]*websocket.Conn),
	}

	// Store channel
	mutex.Lock()
	channels[channelID] = channel
	mutex.Unlock()

	fmt.Printf("Created channel with ID: %s\n", channelID)

	// Respond
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CreateChannelResponse{
		ChannelID: channelID,
	})
}

func joinChannel(w http.ResponseWriter, r *http.Request) {
	// Extract channel ID from URL
	channelID := r.URL.Path[len("/channel/"):]

	// Get password from query
	password := r.URL.Query().Get("password")

	// Find channel
	mutex.Lock()
	channel, exists := channels[channelID]
	mutex.Unlock()

	if !exists || channel.Password != password {
		http.Error(w, "Invalid channel ID or password", http.StatusUnauthorized)
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade: %v", err)
		return
	}

	// Generate client ID
	clientID := uuid.New().String()

	// Add client to channel
	channel.Mutex.Lock()
	channel.Clients[clientID] = conn
	channel.Mutex.Unlock()

	fmt.Printf("Client %s joined channel: %s\n", clientID, channelID)

	// Handle WebSocket messages
	go handleMessages(conn, channel, clientID)
}

func handleMessages(conn *websocket.Conn, channel *Channel, clientID string) {
	// Clean up on exit
	defer func() {
		conn.Close()
		channel.Mutex.Lock()
		delete(channel.Clients, clientID)
		channel.Mutex.Unlock()
		fmt.Printf("Client %s left channel: %s\n", clientID, channel.ID)
	}()

	// Process messages
	for {
		// Read message
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// Broadcast to other clients
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
