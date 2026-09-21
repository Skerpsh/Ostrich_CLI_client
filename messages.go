package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Message struct {
	ID             string `json:"id"`
	ChatID         string `json:"chat_id"`
	SenderID       string `json:"sender_id"`
	SenderUsername string `json:"sender_username"`
	Content        string `json:"content"`
	CreatedAt      string `json:"created_at"`
}

type MessagesResponse struct {
	Messages []Message `json:"messages"`
}

func getMessages(token string, chatID string) ([]Message, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		serverURL+"/api/chats/"+chatID+"/messages",
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
		return nil, fmt.Errorf("failed to get messages: HTTP %d", resp.StatusCode)
	}

	var result MessagesResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Messages, nil
}
