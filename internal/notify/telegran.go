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

type TelegramNotifier struct {
	botToken string
	chatID   string
	client   *http.Client
	log      logr.Logger
}

func (t *TelegramNotifier) Success(ctx context.Context, msg string) error {
	text := fmt.Sprintf("✅ *Backup concluído com sucesso*\n\n%s", msg)
	return t.sendMessage(ctx, text)
}

func (t *TelegramNotifier) Error(ctx context.Context, errMsg string) error {
	text := fmt.Sprintf("❌ *Falha no backup*\n\nDetalhes do erro:\n%s", errMsg)
	return t.sendMessage(ctx, text)
}

func (t *TelegramNotifier) sendMessage(ctx context.Context, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

	payload := map[string]string{
		"chat_id": t.chatID,
		"text":    text,
	}

	body, err := json.Marshal(payload)
	if err != nil {

		return fmt.Errorf("sendMessage: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		t.log.Errorf("Telegram failed: %d", resp.StatusCode)
		return fmt.Errorf("telegram API returned status %s", resp.Status)
	}

	return nil
}

func NewTelegramNotifier(botToken, chatID string, log logr.Logger) Notifier {
	return &TelegramNotifier{
		botToken: botToken,
		chatID:   chatID,
		client:   &http.Client{Timeout: 10 * time.Second},
		log:      log,
	}
}
