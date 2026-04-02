package main

import (
	"log"

	"github.com/panjf2000/gnet/v2"
	"github.com/gliedabrennung/messenger-core/internal/messenger"
)

func main() {
	hub := messenger.NewHub()
	go hub.Run()

	server := messenger.NewWsServer(hub)

	log.Println("Server starting on :8081")
	err := gnet.Run(server, "tcp://:8081", gnet.WithMulticore(true))
	if err != nil {
		log.Fatal(err)
	}
}
