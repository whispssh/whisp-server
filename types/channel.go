package types

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Channel struct {
	ID       string
	Password string
	Clients  map[string]*websocket.Conn
	Mutex    sync.Mutex
}

type CreateChannelRequest struct {
	Password string `json:"password"`
}

type CreateChannelResponse struct {
	ChannelID string `json:"channel_id"`
}
