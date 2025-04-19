package main

import (
	"fmt"
	"log"
	"net/http"
	"whispssh-server/handlers"
	"whispssh-server/middlewares"

	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()
	r.Use(middlewares.Logger)

	r.Post("/channel", handlers.CreateChannel)
	r.Get("/channel/{channelID}", handlers.JoinChannel)

	addr := "0.0.0.0:3000"
	fmt.Printf("Server running on http://%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
