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
	Group    string
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
	Group    string `json:"group"`
	Action   string `json:"action"`
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading connection:", err)
		return
	}
	defer conn.Close()

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

		switch msg.Action {
		case "all":
			broadcast <- msg
		case "sendMessage":
			sendToGroup(client, msg.Target, msg.Message)
		case "joinGroup":
			joinGroup(client, msg.Group)
			replyMessage := Message{
				Username: "Server",
				Message:  fmt.Sprintf("You joined group %s", msg.Group),
			}
			err := client.conn.WriteJSON(replyMessage)
			if err != nil {
				fmt.Println("Error sending joinGroup confirmation:", err)
			}
		default:
			fmt.Println("Unknown action:", msg.Action)
		}
	}
}
func joinGroup(client *client, group string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	client.Group = group
	fmt.Printf("User %s joined group %s\n", client.username, group)
}

func sendToGroup(sender *client, targetGroup, content string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for client := range clients {
		if client != sender && client.Group == targetGroup {
			err := client.conn.WriteJSON(Message{
				Username: sender.username,
				Message:  content,
				Target:   targetGroup,
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

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

// the below is to register the new user:
// {
//     "action": "setUsername",
//     "username": "User4"
// }

// the below is to add user in group1 after registration in postman:
// {
// 	"action": "joinGroup",
// 	"username": "User2",
// 	"group": "group1"
// }

// the below is to send meg to all the users in postman:
// {
//     "action": "all",
//     "content": "Hello, everyone!",
//     "username": "YourUsername"
// }

// now below is to send the message to the group1:
// {
//     "action": "sendMessage",
//     "content": "Hello, Group1!",
//     "username": "User1",
//     "target": "group1"
// }
