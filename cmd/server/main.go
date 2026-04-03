package main

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/hertz-contrib/websocket"
)

var upgrader = websocket.HertzUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(ctx *app.RequestContext) bool {
		return true
	},
}

var addr = ":8080"

func serveHome(_ context.Context, c *app.RequestContext) {
	if string(c.URI().Path()) != "/" {
		hlog.Error("Not found", http.StatusNotFound)
		return
	}
	if !c.IsGet() {
		hlog.Error("Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c.HTML(http.StatusOK, "home.html", nil)
}

func main() {
	go hub.Run()
	h := server.Default(server.WithHostPorts(addr))
	h.LoadHTMLGlob("home.html")

	h.GET("/", serveHome)
	h.GET("/ws", adaptor.HertzHandler(http.HandlerFunc(serveWs)))

	h.Spin()
}
