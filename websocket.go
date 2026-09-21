package main

import (
	"fmt"

	"github.com/gorilla/websocket"
)

const websocketURL = "ws://62.238.111.55:3000/ws"

type WSMessage struct {
	Type     string  `json:"type"`
	ChatID   string  `json:"chatId,omitempty"`
	Message  *Message `json:"message,omitempty"`
	Error    string  `json:"error,omitempty"`
	Username string  `json:"username,omitempty"`
}

func connectWebSocket(token string) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(
		websocketURL+"?token="+token,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func readWebSocketMessages(conn *websocket.Conn, chatID string) {
	for {
		var message WSMessage

		if err := conn.ReadJSON(&message); err != nil {
			fmt.Println()
			fmt.Println("WebSocket closed:", err)
			return
		}

		switch message.Type {
		case "connected":
			fmt.Printf("\nConnected to Ostrich as %s\n", message.Username)

			if err := conn.WriteJSON(map[string]string{
				"type":   "join",
				"chatId": chatID,
			}); err != nil {
				fmt.Println("Join error:", err)
				return
			}

		case "joined":
			fmt.Printf("\nJoined chat\n")
			fmt.Print("> ")

		case "message":
			if message.Message != nil {
				fmt.Printf(
					"\n%s: %s\n",
					message.Message.SenderUsername,
					message.Message.Content,
				)
				fmt.Print("> ")
			}

		case "error":
			fmt.Printf("\nServer error: %s\n", message.Error)
			fmt.Print("> ")
		}
	}
}
