package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const serverURL = "https://62.238.111.55.nip.io"

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

func registerWithCredentials(username, password string) (*LoginResponse, error) {
	payload := map[string]string{
		"username": username,
		"password": password,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to encode registration request: %w",
			err,
		)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		serverURL+"/api/auth/register",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create registration request: %w",
			err,
		)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"server connection failed: %w",
			err,
		)
	}

	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read registration response: %w",
			err,
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var serverError struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}

		if json.Unmarshal(responseBody, &serverError) == nil {
			if serverError.Message != "" {
				return nil, fmt.Errorf("%s", serverError.Message)
			}

			if serverError.Error != "" {
				return nil, fmt.Errorf("%s", serverError.Error)
			}
		}

		return nil, fmt.Errorf(
			"registration failed: HTTP %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(responseBody)),
		)
	}

	// Registration succeeded.
	// Backend does not return a token, so login automatically.
	return loginWithCredentials(username, password)
}

func authRequest(
	endpoint string,
	username string,
	password string,
) (*LoginResponse, error) {
	payload := map[string]string{
		"username": username,
		"password": password,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to encode request: %w",
			err,
		)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		serverURL+endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create request: %w",
			err,
		)
	}

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"server connection failed: %w",
			err,
		)
	}

	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read server response: %w",
			err,
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var serverError struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}

		if json.Unmarshal(responseBody, &serverError) == nil {
			if serverError.Message != "" {
				return nil, fmt.Errorf(
					"%s",
					serverError.Message,
				)
			}

			if serverError.Error != "" {
				return nil, fmt.Errorf(
					"%s",
					serverError.Error,
				)
			}
		}

		return nil, fmt.Errorf(
			"request failed: HTTP %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(responseBody)),
		)
	}

	var result LoginResponse

	if err := json.Unmarshal(
		responseBody,
		&result,
	); err != nil {
		return nil, fmt.Errorf(
			"invalid server response: %w",
			err,
		)
	}

	if result.Token == "" {
		return nil, fmt.Errorf(
			"server did not return authentication token",
		)
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
		return nil, fmt.Errorf(
			"failed to create chats request: %w",
			err,
		)
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+token,
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get chats: %w",
			err,
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf(
			"failed to get chats: HTTP %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var result ChatsResponse

	if err := json.NewDecoder(
		resp.Body,
	).Decode(&result); err != nil {
		return nil, fmt.Errorf(
			"failed to decode chats response: %w",
			err,
		)
	}

	return result.Chats, nil
}
