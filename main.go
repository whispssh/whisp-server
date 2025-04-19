package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Channel struct {
	ID       string
	Password string
	Clients  map[string]*websocket.Conn
	Mutex    sync.Mutex
}

var (
	channels = make(map[string]*Channel)
	mutex    sync.Mutex
)

type CreateChannelRequest struct {
	Password string `json:"password"`
}

type CreateChannelResponse struct {
	ChannelID string `json:"channel_id"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	r := chi.NewRouter()
	r.Use(logger)

	r.Post("/channel", createChannel)
	r.Get("/channel/{channelID}", joinChannel)

	addr := "0.0.0.0:3000"
	fmt.Printf("Server running on http://%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func createChannel(w http.ResponseWriter, r *http.Request) {
	var req CreateChannelRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid request"})
		return
	}

	channelID := uuid.New().String()
	channel := &Channel{
		ID:       channelID,
		Password: req.Password,
		Clients:  make(map[string]*websocket.Conn),
	}

	mutex.Lock()
	channels[channelID] = channel
	mutex.Unlock()

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, CreateChannelResponse{ChannelID: channelID})
}

func joinChannel(w http.ResponseWriter, r *http.Request) {
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

func handleMessages(conn *websocket.Conn, channel *Channel, clientID string) {
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

type statusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.Status = code
	r.ResponseWriter.WriteHeader(code)
}

func logger(next http.Handler) http.Handler {
	styles := map[string]string{
		"reset": "\033[0m",
		"bold":  "\033[1m",

		"blue":    "\033[34m", // GET
		"cyan":    "\033[36m", // POST
		"yellow":  "\033[33m", // PUT
		"magenta": "\033[35m", // DELETE
		"white":   "\033[37m",

		"green": "\033[32m", // 2xx
		"warn":  "\033[33m", // 4xx
		"red":   "\033[31m", // 5xx
		"gray":  "\033[90m",
	}

	methodColor := func(method string) string {
		switch method {
		case "GET":
			return styles["blue"]
		case "POST":
			return styles["cyan"]
		case "PUT":
			return styles["yellow"]
		case "DELETE":
			return styles["magenta"]
		default:
			return styles["white"]
		}
	}

	statusColor := func(code int) string {
		switch {
		case code >= 200 && code < 300:
			return styles["green"]
		case code >= 400 && code < 500:
			return styles["warn"]
		case code >= 500:
			return styles["red"]
		default:
			return styles["white"]
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, Status: http.StatusOK}
		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		method := fmt.Sprintf("%s%s%s%s", styles["bold"], methodColor(r.Method), r.Method, styles["reset"])
		status := fmt.Sprintf("%s%d%s", statusColor(rec.Status), rec.Status, styles["reset"])
		latency := fmt.Sprintf("%s%v%s", styles["gray"], duration, styles["reset"])
		path := r.URL.Path
		remote := fmt.Sprintf("%s%s%s", styles["bold"], r.RemoteAddr, styles["reset"])

		log.Printf("[%s] %s %s - %s from %s", method, status, path, latency, remote)
	})
}
