package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	ID   string
	Conn *websocket.Conn
}

type Group struct {
	ID      string
	Clients map[string]*Client
}

type ChatServer struct {
	Groups map[string]*Group
	sync.Mutex
}

func NewChatServer() *ChatServer {
	return &ChatServer{
		Groups: make(map[string]*Group),
	}
}

func (server *ChatServer) addClientToGroup(groupID string, client *Client) {
	server.Lock()
	defer server.Unlock()

	if _, ok := server.Groups[groupID]; !ok {
		server.Groups[groupID] = &Group{
			ID:      groupID,
			Clients: make(map[string]*Client),
		}
	}
	server.Groups[groupID].Clients[client.ID] = client
}

func (server *ChatServer) removeClientFromGroup(groupID string, clientID string) {
	server.Lock()
	defer server.Unlock()

	if group, ok := server.Groups[groupID]; ok {
		delete(group.Clients, clientID)
		if len(group.Clients) == 0 {
			delete(server.Groups, groupID)
		}
	}
}

func (server *ChatServer) broadcastToGroup(groupID string, senderID string, message string) {
	server.Lock()
	defer server.Unlock()

	if group, ok := server.Groups[groupID]; ok {
		for id, client := range group.Clients {
			if id != senderID { // Xabar yuborgan foydalanuvchiga qaytarmaslik uchun
				err := client.Conn.WriteMessage(websocket.TextMessage, []byte(message))
				if err != nil {
					log.Printf("Error sending message to client %s: %v", client.ID, err)
					client.Conn.Close()
					delete(group.Clients, client.ID)
				}
			}
		}
	}
}

func (server *ChatServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading to WebSocket:", err)
		return
	}
	defer conn.Close()

	clientID := r.URL.Query().Get("client_id")
	groupID := r.URL.Query().Get("group_id")

	client := &Client{
		ID:   clientID,
		Conn: conn,
	}

	server.addClientToGroup(groupID, client)
	defer server.removeClientFromGroup(groupID, client.ID)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}

		log.Printf("Message from client %s in group %s: %s", clientID, groupID, string(message))
		server.broadcastToGroup(groupID, clientID, fmt.Sprintf("%s: %s", clientID, string(message)))
	}
}

func main() {
	server := NewChatServer()

	http.HandleFunc("/ws", server.handleWebSocket)

	log.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
