package messaging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type BotClient struct {
	httpClient *http.Client
	botURL     string
}

type SendMessageRequest struct {
	ChatID   int64  `json:"chat_id"`
	Text     string `json:"text"`
	TicketID string `json:"ticket_id,omitempty"`
}

func NewBotClient(botURL string) *BotClient {
	return &BotClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		botURL:     botURL,
	}
}

func (bc *BotClient) SendMessage(chatID int64, text, ticketID string) error {
	reqBody := SendMessageRequest{
		ChatID:   chatID,
		Text:     text,
		TicketID: ticketID,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal send message: %w", err)
	}

	url := fmt.Sprintf("%s/send-message", bc.botURL)
	resp, err := bc.httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("post to bot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bot returned status %d", resp.StatusCode)
	}
	return nil
}
