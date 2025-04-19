package routers

import (
	"net/http"
	"whispssh-server/handlers"

	"github.com/go-chi/chi/v5"
)

func ChannelRouter() http.Handler {
	r := chi.NewRouter()

	r.Post("/", handlers.CreateChannel)
	r.Get("/{channelID}", handlers.JoinChannel)

	return r
}
