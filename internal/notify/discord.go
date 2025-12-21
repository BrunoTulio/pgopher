package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/BrunoTulio/logr"
)

type DiscordNotifier struct {
	webhookURL string
	log        logr.Logger
	client     *http.Client
}

func (d *DiscordNotifier) Success(ctx context.Context, msg string) error {
	msg = fmt.Sprintf("✅ **Backup Success** `%s`\n`", msg)
	return d.send(ctx, msg)
}

func (d *DiscordNotifier) Error(ctx context.Context, errMsg string) error {
	errMsg = fmt.Sprintf("❌ **Backup Failed** `%s`\n``````", errMsg)
	return d.send(ctx, errMsg)
}

func (d *DiscordNotifier) send(ctx context.Context, msg string) error {
	type Payload struct {
		Content string `json:"content"`
	}
	payload := Payload{Content: msg}
	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", d.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		d.log.Errorf("Discord webhook failed: %d", resp.StatusCode)
		return fmt.Errorf("status: %d", resp.StatusCode)
	}

	return nil
}

func NewDiscord(webhookURL string, log logr.Logger) Notifier {
	return &DiscordNotifier{
		webhookURL: webhookURL,
		log:        log,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}
