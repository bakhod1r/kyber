package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// APIBase is the Telegram Bot API endpoint.
const APIBase = "https://api.telegram.org"

// Bot is a minimal Telegram Bot API client (sendMessage, getUpdates).
type Bot struct {
	token, base string
	http        *http.Client
}

func NewBot(token, base string) *Bot {
	if base == "" {
		base = APIBase
	}
	return &Bot{token: token, base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 70 * time.Second}}
}

func (b *Bot) call(ctx context.Context, method string, in, out any) error {
	body, _ := json.Marshal(in)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.base+"/bot"+b.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := b.http.Do(req)
	if err != nil {
		// The URL holds the bot token: never let it reach logs through the error.
		return fmt.Errorf("telegram %s: %w", method, errors.Unwrap(err))
	}
	defer res.Body.Close()
	var env struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		return fmt.Errorf("telegram %s: %w", method, err)
	}
	if !env.OK {
		return fmt.Errorf("telegram %s: %s", method, env.Description)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(env.Result, out)
}

// SendMessage sends plain text to a chat.
func (b *Bot) SendMessage(ctx context.Context, chatID, text string) error {
	return b.call(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": text}, nil)
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type Update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Text string `json:"text"`
		From User   `json:"from"`
		Chat struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
	} `json:"message"`
}

// Updates long-polls for new updates after offset.
func (b *Bot) Updates(ctx context.Context, offset int64, wait time.Duration) ([]Update, error) {
	var out []Update
	err := b.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": int(wait.Seconds()), "allowed_updates": []string{"message"}}, &out)
	return out, err
}

// StartHandler receives "/start <payload>" from a private chat.
type StartHandler func(ctx context.Context, payload string, from User, chatID string) error

// Poll long-polls the bot until ctx ends, passing /start commands to onStart.
// Run a single poller per bot token (Telegram allows one getUpdates consumer).
func (b *Bot) Poll(ctx context.Context, log *slog.Logger, wait, backoff time.Duration, onStart StartHandler) {
	var offset int64
	for ctx.Err() == nil {
		updates, err := b.Updates(ctx, offset, wait)
		if err != nil {
			if ctx.Err() == nil {
				log.WarnContext(ctx, "telegram poll failed", "err", err)
				select {
				case <-ctx.Done():
				case <-time.After(backoff):
				}
			}
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			m := u.Message
			if m == nil || m.Chat.Type != "private" {
				continue
			}
			cmd, payload, _ := strings.Cut(strings.TrimSpace(m.Text), " ")
			if cmd != "/start" {
				continue
			}
			if err := onStart(ctx, strings.TrimSpace(payload), m.From, strconv.FormatInt(m.Chat.ID, 10)); err != nil {
				log.InfoContext(ctx, "telegram /start not handled", "err", err)
			}
		}
	}
}

// DisplayName is "First Last", or the username.
func (u User) DisplayName() string {
	if n := strings.TrimSpace(u.FirstName + " " + u.LastName); n != "" {
		return n
	}
	return u.Username
}
