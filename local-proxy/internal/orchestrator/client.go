package orchestrator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

type ProcessTicketRequest struct {
	Text         string                 `json:"text"`
	DispatcherID string                 `json:"dispatcher_id"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type ProcessTicketResponse struct {
	TicketID       string                 `json:"ticket_id"`
	Classification map[string]interface{} `json:"classification,omitempty"`
	SuggestedTeam  string                 `json:"suggested_team"`
	Status         string                 `json:"status"`
	Plan           string                 `json:"plan,omitempty"`
	ExecutionLog   []string               `json:"execution_log,omitempty"`
}

func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    baseURL,
		apiKey:     apiKey,
	}
}

func (c *Client) ProcessTicket(text, dispatcherID string) (*ProcessTicketResponse, error) {
	reqBody := ProcessTicketRequest{
		Text:         text,
		DispatcherID: dispatcherID,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/process-ticket", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to orchestrator: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("orchestrator returned status %d", resp.StatusCode)
	}

	var result ProcessTicketResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}
