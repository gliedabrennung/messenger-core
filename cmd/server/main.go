package main

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/gliedabrennung/messenger-core/internal/controller/http"
	"github.com/gliedabrennung/messenger-core/internal/messenger"
)

var addr = ":8080"

func main() {
	go messenger.StartHub()

	h := server.Default(server.WithHostPorts(addr))
	h.LoadHTMLGlob("home.html")

	http.SetupRouter(h)

	h.Spin()
}
