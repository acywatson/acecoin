package main

import (
	"github.com/gorilla/websocket"
	"time"
	"log"
	"bytes"
	"net/http"
	"net"
	"encoding/json"
	"fmt"
)

// declare our types

type Client struct {
	hub *P2PServer
	conn *websocket.Conn
	send chan []byte
}

type P2PServer struct {
	clients map [*Client]bool
	register chan *Client
	unregister chan *Client
	broadcast chan []byte
	queryLatest chan *Client
	queryAll chan *Client
	chainResponse chan []Block
}

type Message struct {
	MessageType float64 `json:"messageType"`
	Data []Block `json:"data"`
}

var messageTypes = map[string]float64 {
	"QUERY_LATEST": 0,
	"QUERY_ALL": 1,
	"BLOCKHAIN": 2,
}

// Server (Hub)

// initialize the P2PServer struct and return a pointer
func getNewP2PServer() *P2PServer {
	return &P2PServer{
		clients: make(map [*Client]bool),
		register: make(chan *Client),
		unregister: make(chan *Client),
		broadcast: make(chan []byte),
		queryLatest: make(chan *Client),
		queryAll: make(chan *Client),
		chainResponse: make(chan []Block),
	}
}

// fire up the server and listen on all the channels
func initializeP2PServer(hub *P2PServer, blockchain *[]Block) {
	for {
		select {
			case client := <-hub.register:
				hub.clients[client] = true
			case client := <-hub.unregister:
				if _, ok := hub.clients[client]; ok {
					delete(hub.clients, client)
					close(client.send)
				}
			case message := <-hub.broadcast:
				for client := range hub.clients {
					select {
						case client.send <-message:
						default:
							close(client.send)
							delete(hub.clients, client)
					}
				}
			case client := <- hub.queryLatest:
				latestBlock := append(make([]Block, 0), getLatestBlock(*blockchain))
				client.send <-createJSONResponse(messageTypes["BLOCKHAIN"],latestBlock)
			case client := <- hub.queryAll:
				client.send <-createJSONResponse(messageTypes["BLOCKHAIN"], *blockchain)
		}
	}
}

// return a list of all currently registered peer nodes
func (h *P2PServer) listPeers() []net.Addr {
	peers := make([]net.Addr, 0)
	for peer := range h.clients {
		addr := peer.conn.RemoteAddr()
		peers = append(peers, addr)
	}
	return peers
}

// Client

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		var messageJSON Message
		if err := json.Unmarshal(message, &messageJSON); err != nil {
			panic(err)
		}
		messageType := messageJSON.MessageType
		switch messageType {
			case messageTypes["QUERY_LATEST"]:
				c.hub.queryLatest <-c
			case messageTypes["QUERY_ALL"]:
				c.hub.queryAll <-c
			case messageTypes["BLOCKHAIN"]:
				handleBlockchainResponse(messageJSON.Data, &blockchain, c)
		default:
				log.Printf("error: %v", "invalid message type.")
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// This method determines what we do when we receive a new chain from another node.
func handleBlockchainResponse(receivedBlocks []Block, heldBlocks *[]Block, c *Client) {
	if len(receivedBlocks) == 0 {
		fmt.Println("Recived a blockchain of length 0")
		return
	}
	latestBlockReceived := getLatestBlock(receivedBlocks)
	latestBlockHeld := getLatestBlock(*heldBlocks)
	if latestBlockReceived.Index > latestBlockHeld.Index {
		if validateNewBlock(latestBlockReceived, latestBlockHeld) {
			c.hub.broadcast <-createJSONResponse(messageTypes["BLOCKHAIN"], *addBlockToChain(heldBlocks, latestBlockReceived))
		} else if len(receivedBlocks) == 1 {
			c.hub.broadcast <-createJSONResponse(messageTypes["QUERY_ALL"], nil)
		} else {
			if newChain := replaceChain(heldBlocks, *heldBlocks, receivedBlocks); newChain != nil {
				c.hub.broadcast <-createJSONResponse(messageTypes["BLOCKHAIN"], *newChain)
			}
		}
	}
}

// Format our message struct for transmission via websocket to our peer(s)
func createJSONResponse(messageType float64, data []Block) []byte {
	response := Message{
		MessageType: messageType,
		Data: data,
	}
	JSONresponse, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}
	return JSONresponse
}

// Handle websocket requests from peers.
func serveWs(hub *P2PServer, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
