package main

import (
	"fmt"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

type Message struct {
	Room  string
	Name  string
	Price string
}

type Hub struct {
	Clients               map[*websocket.Conn]bool
	ClientRegisterChannel chan *websocket.Conn
	ClientRemovalChannel  chan *websocket.Conn
	BroadcastMessage      chan Message
	Rooms                 map[string]map[*websocket.Conn]bool
}

func (h *Hub) run() {
	for {
		select {
		case conn := <-h.ClientRegisterChannel:
			h.Clients[conn] = true
		case conn := <-h.ClientRemovalChannel:
			delete(h.Clients, conn)
			h.leaveRoom(conn)
		case msg := <-h.BroadcastMessage:
			h.broadcastToRoom(msg)
		}
	}
}

func main() {
	h := &Hub{
		Clients:               make(map[*websocket.Conn]bool),
		ClientRegisterChannel: make(chan *websocket.Conn),
		ClientRemovalChannel:  make(chan *websocket.Conn),
		BroadcastMessage:      make(chan Message),
		Rooms:                 make(map[string]map[*websocket.Conn]bool),
	}
	go h.run()
	app := fiber.New()

	app.Use("/ws", AllowUpgrade)
	app.Use("/ws/bid", websocket.New(BidPrice(h)))

	app.Listen(":3000")

}

func AllowUpgrade(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		c.Locals("allowed", true)
		return c.Next()
	}
	return fiber.ErrUpgradeRequired
}

func BidPrice(h *Hub) func(conn *websocket.Conn) {
	return func(conn *websocket.Conn) {
		defer func() {
			h.ClientRemovalChannel <- conn
			_ = conn.Close()
		}()

		name := conn.Query("name", "")
		room := conn.Query("room", "default") // Get the room name from the query string

		h.ClientRegisterChannel <- conn
		h.joinRoom(conn, room) // Join the specified room

		for {
			messageType, price, err := conn.ReadMessage()
			if err != nil {
				return
			}

			if messageType == websocket.TextMessage {
				h.BroadcastMessage <- Message{
					Room:  room,
					Name:  name,
					Price: string(price),
				}
			}

		}
	}
}

func (h *Hub) leaveRoom(conn *websocket.Conn) {
	for room, connections := range h.Rooms {
		if _, ok := connections[conn]; ok {
			delete(connections, conn)
			fmt.Printf("Client left room %s\n", room)
		}
	}
}

func (h *Hub) broadcastToRoom(msg Message) {
	connections, ok := h.Rooms[msg.Room]
	if ok {
		for conn := range connections {
			_ = conn.WriteJSON(msg)
		}
	}
}

func (h *Hub) joinRoom(conn *websocket.Conn, room string) {
	if _, ok := h.Rooms[room]; !ok {
		h.Rooms[room] = make(map[*websocket.Conn]bool)
	}
	h.Rooms[room][conn] = true
}
