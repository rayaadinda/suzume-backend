package websocket

import (
	"encoding/json"
	"log"
	"sync"
)

// Hub maintains the set of active clients, user sockets, and room subscriptions
type Hub struct {
	mu sync.RWMutex

	// Registered clients
	clients map[*Client]bool

	// User ID -> set of connected clients
	userClients map[string]map[*Client]bool

	// Room ID (e.g. "channel:123" or "list:456") -> set of subscribed clients
	rooms map[string]map[*Client]bool

	// Inbound messages from the clients
	broadcast chan []byte

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		broadcast:   make(chan []byte, 256),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		clients:     make(map[*Client]bool),
		userClients: make(map[string]map[*Client]bool),
		rooms:       make(map[string]map[*Client]bool),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if client.userID != "" {
				if h.userClients[client.userID] == nil {
					h.userClients[client.userID] = make(map[*Client]bool)
				}
				h.userClients[client.userID][client] = true
			}
			h.mu.Unlock()
			log.Printf("Client connected (%s). Total clients: %d", client.userID, len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				if client.userID != "" && h.userClients[client.userID] != nil {
					delete(h.userClients[client.userID], client)
					if len(h.userClients[client.userID]) == 0 {
						delete(h.userClients, client.userID)
					}
				}
				// Remove client from all rooms
				for roomID := range h.rooms {
					delete(h.rooms[roomID], client)
					if len(h.rooms[roomID]) == 0 {
						delete(h.rooms, roomID)
					}
				}
				close(client.send)
				log.Printf("Client disconnected (%s). Total clients: %d", client.userID, len(h.clients))
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// JoinRoom adds a client to a specific room
func (h *Hub) JoinRoom(roomID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]bool)
	}
	h.rooms[roomID][client] = true
	log.Printf("Client %s joined room %s", client.userID, roomID)
}

// LeaveRoom removes a client from a specific room
func (h *Hub) LeaveRoom(roomID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[roomID] != nil {
		delete(h.rooms[roomID], client)
		if len(h.rooms[roomID]) == 0 {
			delete(h.rooms, roomID)
		}
		log.Printf("Client %s left room %s", client.userID, roomID)
	}
}

// BroadcastToRoom dispatches a message to all clients subscribed to a room
func (h *Hub) BroadcastToRoom(roomID string, msgType MessageType, payload interface{}) error {
	message, err := NewMessage(msgType, roomID, payload)
	if err != nil {
		return err
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.rooms[roomID]
	if !ok {
		return nil
	}

	for client := range clients {
		select {
		case client.send <- messageBytes:
		default:
		}
	}

	return nil
}

// SendToUser dispatches a message directly to all sockets connected for a specific user
func (h *Hub) SendToUser(userID string, msgType MessageType, payload interface{}) error {
	message, err := NewMessage(msgType, "", payload)
	if err != nil {
		return err
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.userClients[userID]
	if !ok {
		return nil
	}

	for client := range clients {
		select {
		case client.send <- messageBytes:
		default:
		}
	}

	return nil
}

// BroadcastTaskUpdate broadcasts a task update message to all connected clients
func (h *Hub) BroadcastTaskUpdate(msgType MessageType, payload interface{}) error {
	message, err := NewTaskMessage(msgType, payload)
	if err != nil {
		return err
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	h.broadcast <- messageBytes
	return nil
}

// GetClientCount returns the number of connected clients
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// RegisterClient registers a new client with the hub
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient unregisters a client from the hub
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}
