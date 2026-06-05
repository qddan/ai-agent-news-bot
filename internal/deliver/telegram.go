package deliver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

// Telegram is a minimal Bot API client built on net/http (no external lib).
type Telegram struct {
	token  string
	chatID string
	client *http.Client
}

// NewTelegram builds a client for the given bot token + target chat.
func NewTelegram(token, chatID string) *Telegram {
	return &Telegram{token: token, chatID: chatID, client: &http.Client{Timeout: 30 * time.Second}}
}

const apiBase = "https://api.telegram.org/bot"

// SendAll sends each message in order with a 1s gap, honoring 429 retry_after.
func (t *Telegram) SendAll(ctx context.Context, messages []string) error {
	for i, msg := range messages {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
			}
		}
		if err := t.sendOne(ctx, msg); err != nil {
			return fmt.Errorf("message %d/%d: %w", i+1, len(messages), err)
		}
	}
	return nil
}

// sendOne POSTs a single message, retrying once on a 429 after retry_after.
func (t *Telegram) sendOne(ctx context.Context, text string) error {
	form := url.Values{}
	form.Set("chat_id", t.chatID)
	form.Set("text", text)
	form.Set("parse_mode", "HTML")
	form.Set("disable_web_page_preview", "true")

	endpoint := apiBase + t.token + "/sendMessage"

	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
			bytes.NewBufferString(form.Encode()))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := t.client.Do(req)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			wait := parseRetryAfter(body)
			log.Printf("[telegram] 429, waiting %s", wait)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		return fmt.Errorf("telegram status %d: %s", resp.StatusCode, string(body))
	}
	return fmt.Errorf("telegram: still rate-limited after retry")
}

// parseRetryAfter pulls parameters.retry_after (seconds) from a 429 body.
func parseRetryAfter(body []byte) time.Duration {
	var r struct {
		Parameters struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal(body, &r); err == nil && r.Parameters.RetryAfter > 0 {
		return time.Duration(r.Parameters.RetryAfter) * time.Second
	}
	return 3 * time.Second
}

// NotifyError best-effort sends a plain-text failure note to the chat.
func (t *Telegram) NotifyError(ctx context.Context, msg string) {
	form := url.Values{}
	form.Set("chat_id", t.chatID)
	form.Set("text", "⚠️ Digest bot lỗi: "+msg)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		apiBase+t.token+"/sendMessage", bytes.NewBufferString(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// GetChatID calls getUpdates and prints the chat IDs found, to help the user
// discover their TELEGRAM_CHAT_ID after messaging the bot once.
func (t *Telegram) GetChatID(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		apiBase+t.token+"/getUpdates", nil)
	if err != nil {
		return "", err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var data struct {
		OK     bool `json:"ok"`
		Result []struct {
			Message struct {
				Chat struct {
					ID    int64  `json:"id"`
					Title string `json:"title"`
					Type  string `json:"type"`
				} `json:"chat"`
			} `json:"message"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("parse getUpdates: %w (body: %s)", err, string(body))
	}
	if !data.OK || len(data.Result) == 0 {
		return "", fmt.Errorf("no updates found — gửi 1 tin nhắn cho bot rồi thử lại (body: %s)", string(body))
	}

	last := ""
	seen := map[int64]bool{}
	for _, u := range data.Result {
		id := u.Message.Chat.ID
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		last = fmt.Sprintf("%d", id)
		log.Printf("[get-chat-id] chat_id=%d type=%s title=%q", id, u.Message.Chat.Type, u.Message.Chat.Title)
	}
	return last, nil
}
