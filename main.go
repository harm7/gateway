package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nats-io/go-nats"
)

type (
	Message struct {
		Method string              `json:"method"`
		Args   []string            `json:"args"`
		Query  map[string][]string `json:"query"`
	}

	Reply struct {
		Error string                 `json:"error"`
		Data  map[string]interface{} `json:"data"`
	}

	Context struct {
		conn       *nats.EncodedConn
		wsUpgrader *websocket.Upgrader
	}
)

func (ctx *Context) httpHandler(w http.ResponseWriter, r *http.Request) {
	tokens := strings.Split(r.URL.Path[1:], "/")
	topic := tokens[0]

	req := Message{Method: r.Method, Args: tokens[1:], Query: r.URL.Query()}
	resp := Reply{}
	err := ctx.conn.Request(topic, &req, &resp, 3*time.Second)
	if err != nil {
		log.Printf("Error: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Reply: %v", resp)
	w.Header().Set("Content-Type", "application/json")
	jsonResp, err := json.Marshal(resp)
	w.Write(jsonResp)
}

func (ctx *Context) wsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Establishing websocket connection")
	tokens := strings.Split(r.URL.Path[1:], "/")
	topic := tokens[1]

	conn, err := ctx.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx.conn.Subscribe(topic, func(reqTopic, respTopic string, msg interface{}) {
		log.Printf("ws update [%s]: %v", reqTopic, msg)
		if err = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Reply: %v", msg))); err != nil {
			log.Printf("Error: %s", err)
			return
		}
	})
}

func main() {
	var ctx Context

	ctx.wsUpgrader = &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}
	ctx.wsUpgrader.CheckOrigin = func(r *http.Request) bool { return true }

	natsHost := os.Getenv("NATS_HOST")
	if len(natsHost) == 0 {
		natsHost = nats.DefaultURL
	}
	log.Printf("NATS: Connecting to %s", natsHost)
	nc, err := nats.Connect(natsHost)
	if err != nil {
		log.Printf("Error: %s", err)
		os.Exit(1)
	}
	ctx.conn, err = nats.NewEncodedConn(nc, "json")
	if err != nil {
		log.Printf("Error: %s", err)
		os.Exit(1)
	}
	log.Printf("NATS: Connected")

	handler := http.NewServeMux()
	handler.HandleFunc("/ws/", ctx.wsHandler)
	handler.HandleFunc("/", ctx.httpHandler)
	log.Printf("Starting HTTP server")
	log.Fatal(http.ListenAndServe(":8081", handler))
}
