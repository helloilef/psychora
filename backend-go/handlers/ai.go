package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type aiRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
	UserType  string `json:"user_type"`
}

type aiResponse struct {
	Response string `json:"response"`
}

func callPythonAI(message string, sessionID string) (string, error) {
	pythonURL := os.Getenv("PYTHON_API_URL") + "/chat"

	payload := aiRequest{
		Message:   message,
		SessionID: sessionID,
		UserType:  "therapist",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(pythonURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("error calling AI service: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result aiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("error parsing AI response: %w", err)
	}

	return result.Response, nil
}
