package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

const serverURL = "http://62.238.111.55:3000"

type LoginResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`

	User struct {
		ID       string `json:"id"`
		LoginID  string `json:"login_id"`
		Username string `json:"username"`
	} `json:"user"`
}

type Chat struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Created  string `json:"created_at"`
	Updated  string `json:"updated_at"`
	UserID   string `json:"user_id"`
	LoginID  string `json:"login_id"`
	Username string `json:"username"`
}

type ChatsResponse struct {
	Chats []Chat `json:"chats"`
}

func login() (*LoginResponse, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	username = strings.TrimSpace(username)

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}

	fmt.Println()

	body, err := json.Marshal(map[string]string{
		"username": username,
		"password": string(passwordBytes),
	})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(
		serverURL+"/api/auth/login",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed: HTTP %d", resp.StatusCode)
	}

	var result LoginResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func getChats(token string) ([]Chat, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		serverURL+"/api/chats",
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get chats: HTTP %d", resp.StatusCode)
	}

	var result ChatsResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Chats, nil
}

func printMessages(messages []Message) {
	for _, message := range messages {
		fmt.Printf("%s: %s\n", message.SenderUsername, message.Content)
	}
}

func connectAndJoin(token, chatID string) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(
		"ws://62.238.111.55:3000/ws?token="+token,
		nil,
	)
	if err != nil {
		return nil, err
	}

	var connected WSMessage

	if err := conn.ReadJSON(&connected); err != nil {
		conn.Close()
		return nil, err
	}

	if connected.Type != "connected" {
		conn.Close()
		return nil, fmt.Errorf("unexpected server response: %s", connected.Type)
	}

	fmt.Printf("Connected to Ostrich as %s\n", connected.Username)

	if err := conn.WriteJSON(map[string]string{
		"type":   "join",
		"chatId": chatID,
	}); err != nil {
		conn.Close()
		return nil, err
	}

	var joined WSMessage

	if err := conn.ReadJSON(&joined); err != nil {
		conn.Close()
		return nil, err
	}

	if joined.Type != "joined" {
		conn.Close()
		return nil, fmt.Errorf("failed to join chat")
	}

	fmt.Println("Joined chat.")

	return conn, nil
}

func listenForMessages(conn *websocket.Conn) {
	for {
		var message WSMessage

		if err := conn.ReadJSON(&message); err != nil {
			fmt.Println()
			fmt.Println("WebSocket disconnected.")
			return
		}

		switch message.Type {
		case "message":
			if message.Message != nil {
				fmt.Printf(
					"\n%s: %s\n> ",
					message.Message.SenderUsername,
					message.Message.Content,
				)
			}

		case "error":
			fmt.Printf("\nServer error: %s\n> ", message.Error)
		}
	}
}

func main() {
	fmt.Println("╔════════════════════╗")
	fmt.Println("║    OSTRICH CLI     ║")
	fmt.Println("╚════════════════════╝")
	fmt.Println()

	result, err := login()
	if err != nil {
		fmt.Println("Login error:", err)
		return
	}

	fmt.Println()
	fmt.Println("Login successful!")
	fmt.Println("Username:", result.User.Username)
	fmt.Println("Login ID:", result.User.LoginID)
	fmt.Println()

	chats, err := getChats(result.Token)
	if err != nil {
		fmt.Println("Chat error:", err)
		return
	}

	if len(chats) == 0 {
		fmt.Println("No chats.")
		return
	}

	fmt.Println("Chats:")
	fmt.Println()

	for i, chat := range chats {
		fmt.Printf("[%d] %s (%s)\n", i+1, chat.Username, chat.LoginID)
	}

	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	var selected Chat

	for {
		fmt.Print("Select chat number: ")

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Input error:", err)
			return
		}

		number, err := strconv.Atoi(strings.TrimSpace(input))
		if err != nil || number < 1 || number > len(chats) {
			fmt.Println("Invalid chat number.")
			continue
		}

		selected = chats[number-1]
		break
	}

	fmt.Println()
	fmt.Println("Selected chat:", selected.Username)
	fmt.Println()

	messages, err := getMessages(result.Token, selected.ID)
	if err != nil {
		fmt.Println("Message error:", err)
		return
	}

	printMessages(messages)

	fmt.Println()

	conn, err := connectAndJoin(result.Token, selected.ID)
	if err != nil {
		fmt.Println("WebSocket error:", err)
		return
	}
	defer conn.Close()

	go listenForMessages(conn)

	fmt.Println()
	fmt.Println("You are now in the chat.")
	fmt.Println("Type a message and press Enter.")
	fmt.Println("Type /exit to leave.")
	fmt.Println()

	for {
		fmt.Print("> ")

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			return
		}

		content := strings.TrimSpace(input)

		if content == "" {
			continue
		}

		if content == "/exit" {
			return
		}

		err = conn.WriteJSON(map[string]string{
			"type":    "message",
			"chatId":  selected.ID,
			"content": content,
		})

		if err != nil {
			fmt.Println("Send error:", err)
			return
		}
	}
}
