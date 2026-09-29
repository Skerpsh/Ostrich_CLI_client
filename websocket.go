package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsHandshakeTimeout = 10 * time.Second
	wsWriteTimeout     = 10 * time.Second
	wsReadLimit        = 1 << 20
)

type WSMessage struct {
	Type     string   `json:"type"`
	ChatID   string   `json:"chatId,omitempty"`
	Message  *Message `json:"message,omitempty"`
	Error    string   `json:"error,omitempty"`
	Username string   `json:"username,omitempty"`
}

// connectWebSocket opens a websocket connection and joins the given chat.
func connectWebSocket(token, chatID string) (*websocket.Conn, error) {
	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: wsHandshakeTimeout,
	}

	wsURL, err := websocketURL()
	if err != nil {
		return nil, err
	}

	// Token goes in a header rather than the URL, so it does not end up
	// in proxy access logs.
	header := http.Header{"Authorization": {"Bearer " + token}}

	conn, resp, err := dialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, errSessionExpired
		}

		return nil, fmt.Errorf("websocket connection failed: %w", err)
	}

	conn.SetReadLimit(wsReadLimit)

	if err := joinChat(conn, chatID); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func joinChat(conn *websocket.Conn, chatID string) error {
	// Bound the handshake so an unresponsive server can't hang the UI.
	if err := conn.SetReadDeadline(time.Now().Add(wsHandshakeTimeout)); err != nil {
		return err
	}

	var connected WSMessage

	if err := conn.ReadJSON(&connected); err != nil {
		return err
	}

	if connected.Type != "connected" {
		return fmt.Errorf(
			"unexpected websocket response: %s",
			connected.Type,
		)
	}

	if err := writeWebSocketJSON(conn, map[string]string{
		"type":   "join",
		"chatId": chatID,
	}); err != nil {
		return err
	}

	var joined WSMessage

	if err := conn.ReadJSON(&joined); err != nil {
		return err
	}

	if joined.Type != "joined" {
		if joined.Error != "" {
			return fmt.Errorf("failed to join chat: %s", joined.Error)
		}

		return fmt.Errorf("failed to join chat")
	}

	return conn.SetReadDeadline(time.Time{})
}

func writeWebSocketJSON(conn *websocket.Conn, v any) error {
	if err := conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout)); err != nil {
		return err
	}

	return conn.WriteJSON(v)
}
