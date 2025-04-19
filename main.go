package main

import (
	"fmt"
	"log"
	"net/http"
	"whispssh-server/middlewares"
	"whispssh-server/routers"

	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()
	r.Use(middlewares.Logger)

	r.Mount("/channel", routers.ChannelRouter())

	addr := "0.0.0.0:3000"
	fmt.Printf("Server running on http://%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
