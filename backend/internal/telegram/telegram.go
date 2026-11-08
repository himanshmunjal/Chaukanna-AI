// Package telegram sends family alerts through a Telegram bot.
//
// WhatsApp Cloud API needs business verification and an open 24-hour session, and
// SMS costs money. A Telegram bot is free and works in minutes:
//
//  1. Create a bot with @BotFather and set TELEGRAM_BOT_TOKEN.
//  2. The dashboard's People page shows a link like t.me/<bot>?start=<code>.
//  3. The family member opens it and taps Start. The code links their chat.
//
// The code is random per person, so knowing a phone number is not enough to
// receive someone's alerts.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client talks to the Telegram Bot API.
type Client struct {
	base string
	http *http.Client

	mu       sync.Mutex
	username string
}

// New creates a client. An empty token disables it.
func New(token string) *Client {
	if token == "" {
		return &Client{}
	}
	return &Client{base: "https://api.telegram.org/bot" + token, http: &http.Client{Timeout: 40 * time.Second}}
}

// NewWithBase is New for tests: base is the full API prefix.
func NewWithBase(base string) *Client {
	return &Client{base: base, http: &http.Client{Timeout: 5 * time.Second}}
}

// Enabled reports whether a bot token is configured.
func (c *Client) Enabled() bool { return c != nil && c.base != "" }

func (c *Client) call(ctx context.Context, method string, payload, out any) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		// Errors from net/http include the URL, and so the bot token: never pass them on.
		return fmt.Errorf("telegram %s: request failed", method)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram %s returned %d: %.200s", method, resp.StatusCode, raw)
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// Send delivers a plain-text message.
//
// A scam alert may be someone's only warning, so a network blip gets retried.
func (c *Client) Send(ctx context.Context, chatID, text string) error {
	var err error
	for attempt, wait := range []time.Duration{0, time.Second, 3 * time.Second} {
		if attempt > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return err
			}
		}
		err = c.call(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": text}, nil)
		if err == nil {
			return nil
		}
	}
	return err
}

// Username returns the bot's @name (needed for the t.me link).
func (c *Client) Username(ctx context.Context) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.username == "" && c.Enabled() {
		var out struct {
			Result struct {
				Username string `json:"username"`
			} `json:"result"`
		}
		if err := c.call(ctx, "getMe", map[string]any{}, &out); err == nil {
			c.username = out.Result.Username
		}
	}
	return c.username
}

// LinkHandler is called for "/start <code>". It returns the reply to send.
type LinkHandler func(chatID, code string) string

// Listen long-polls for /start messages until ctx ends.
func (c *Client) Listen(ctx context.Context, onLink LinkHandler) {
	if !c.Enabled() {
		return
	}
	offset := 0
	for ctx.Err() == nil {
		var out struct {
			Result []struct {
				UpdateID int `json:"update_id"`
				Message  struct {
					Text string `json:"text"`
					Chat struct {
						ID int64 `json:"id"`
					} `json:"chat"`
				} `json:"message"`
			} `json:"result"`
		}
		err := c.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": 25}, &out)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("telegram: %v", err)
				time.Sleep(10 * time.Second)
			}
			continue
		}
		for _, u := range out.Result {
			offset = u.UpdateID + 1
			fields := strings.Fields(u.Message.Text)
			if len(fields) != 2 || fields[0] != "/start" {
				continue
			}
			chatID := fmt.Sprint(u.Message.Chat.ID)
			if reply := onLink(chatID, fields[1]); reply != "" {
				if err := c.Send(ctx, chatID, reply); err != nil {
					log.Printf("telegram: %v", err)
				}
			}
		}
	}
}
