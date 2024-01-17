package main

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type client struct {
	conn     *websocket.Conn
	username string
}

var (
	clients   = make(map[*client]bool)
	clientsMu sync.Mutex
	broadcast = make(chan Message)
)

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
	Target   string `json:"target"`
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading connection:", err)
		return
	}
	defer conn.Close()

	// Prompt the user for their username
	username := promptUsername(conn)

	client := &client{
		conn:     conn,
		username: username,
	}

	clientsMu.Lock()
	clients[client] = true
	clientsMu.Unlock()

	defer func() {
		clientsMu.Lock()
		delete(clients, client)
		clientsMu.Unlock()
		fmt.Println("User disconnected:", client.username)
	}()

	fmt.Println("User connected:", client.username)

	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			fmt.Println("Error reading message:", err)
			break
		}

		fmt.Printf("Received message from %s: %s\n", client.username, msg.Message)

		switch msg.Target {
		case "all":
			// Broadcast the message to all users
			broadcast <- msg
		case "group1":
			// Send the message to users in the "group1" group
			sendToGroup(client, "group1", msg.Message)
		default:
			fmt.Println("Unknown target:", msg.Target)
		}
	}
}

func sendToGroup(sender *client, group, content string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	if group == "" {
		return
	}

	for client := range clients {
		if client != sender {
			err := client.conn.WriteJSON(Message{
				Username: sender.username,
				Message:  content,
			})
			if err != nil {
				fmt.Println("Error sending message to", client.username, ":", err)
			}
		}
	}
}

func promptUsername(conn *websocket.Conn) string {
	conn.WriteMessage(websocket.TextMessage, []byte("hello:"))
	_, usernameBytes, err := conn.ReadMessage()
	if err != nil {
		fmt.Println("Error reading username:", err)
		return "Anonymous"
	}
	username := string(usernameBytes)
	fmt.Println("User set username:", username)
	return username
}

func handleMessages() {
	for {
		msg := <-broadcast
		clientsMu.Lock()
		for client := range clients {
			err := client.conn.WriteJSON(msg)
			if err != nil {
				fmt.Println("Error broadcasting message to", client.username, ":", err)
			}
		}
		clientsMu.Unlock()
	}
}

func main() {
	http.HandleFunc("/ws", handleConnections)

	go handleMessages()

	// Start the server on localhost:8080
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
