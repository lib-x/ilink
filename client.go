package ilink

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const (
	defaultBase           = "https://ilinkai.weixin.qq.com"
	defaultLongPollMs     = 35_000
	channelVersion        = "1.0.2"
)

// Client is a WeChat iLink bot client. Create one with [NewClient].
//
// A Client is safe for concurrent use by multiple goroutines.
type Client struct {
	token   Token
	base    string
	http    *http.Client
	logger  *slog.Logger
}

// Option configures a [Client]. Pass options to [NewClient].
type Option func(*Client)

// WithHTTPClient replaces the default HTTP client used for all API calls.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.http = hc }
}

// WithLogger sets the structured logger used by [Client.ListenAndServe] to
// report polling errors and handler panics. The default is [slog.Default].
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) { c.logger = l }
}

// WithBaseURL overrides the iLink API base URL. Useful for testing against a
// mock server. The Token.BaseURL returned during login takes precedence.
func WithBaseURL(u string) Option {
	return func(c *Client) { c.base = u }
}

// NewClient creates a new iLink [Client] authenticated with the given [Token].
//
//	token, _ := login.Wait(ctx)
//	client := ilink.NewClient(token)
func NewClient(token Token, opts ...Option) *Client {
	base := defaultBase
	if token.BaseURL != "" {
		base = token.BaseURL
	}
	c := &Client{
		token:  token,
		base:   base,
		http:   &http.Client{Timeout: 45 * time.Second}, // > max long-poll hold
		logger: slog.Default(),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// ListenAndServe starts the long-poll loop, calling h.ServeMessage for every
// inbound message. It blocks until ctx is cancelled or a non-recoverable error
// occurs, and returns ctx.Err() on clean shutdown.
//
// Handler errors are logged via the client's logger and do not stop the loop.
// Use ctx cancellation to shut down gracefully.
//
//	err := client.ListenAndServe(ctx, ilink.HandlerFunc(func(ctx context.Context, msg *ilink.Message) error {
//	    return client.Reply(ctx, msg, ilink.TextMessage("pong"))
//	}))
func (c *Client) ListenAndServe(ctx context.Context, h Handler) error {
	var cursor string
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		msgs, next, err := c.getUpdates(ctx, cursor)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logger.ErrorContext(ctx, "ilink: poll error", slog.Any("err", err))
			// Back off briefly on transient errors.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		}

		if next != "" {
			cursor = next
		}

		for _, raw := range msgs {
			msg := convertMessage(raw)
			if err := h.ServeMessage(ctx, msg); err != nil {
				c.logger.ErrorContext(ctx, "ilink: handler error",
					slog.String("from", msg.From),
					slog.Any("err", err),
				)
			}
		}
	}
}

// Send delivers an [OutboundMessage] to a WeChat user.
func (c *Client) Send(ctx context.Context, msg *OutboundMessage) error {
	if msg.To == "" {
		return fmt.Errorf("ilink: Send: OutboundMessage.To must not be empty")
	}
	if len(msg.Items) == 0 {
		return fmt.Errorf("ilink: Send: OutboundMessage.Items must not be empty")
	}

	body := sendMessageRequest{
		Msg: sendMessageBody{
			ToUserID:     msg.To,
			GroupID:      msg.GroupID,
			MessageType:  2, // outbound
			MessageState: 2, // finish
			ContextToken: msg.ContextToken,
			ItemList:     marshalItems(msg.Items),
		},
	}
	var resp struct {
		Ret int `json:"ret"`
	}
	if err := c.postJSON(ctx, "/ilink/bot/sendmessage", body, &resp); err != nil {
		return fmt.Errorf("ilink: Send: %w", err)
	}
	return checkRet("sendmessage", resp.Ret)
}

// Reply is a convenience wrapper around [Client.Send] that copies the
// ContextToken from an inbound message so WeChat routes the reply correctly.
//
//	return client.Reply(ctx, msg, ilink.TextMessage("pong"))
func (c *Client) Reply(ctx context.Context, to *Message, items ...Item) error {
	return c.Send(ctx, &OutboundMessage{
		To:           to.From,
		GroupID:      to.GroupID,
		ContextToken: to.ContextToken,
		Items:        items,
	})
}

// SendTyping sends a "typing…" indicator to the given user.
// status=true starts the indicator; status=false stops it.
func (c *Client) SendTyping(ctx context.Context, userID string, status bool) error {
	cfg, err := c.getConfig(ctx, userID)
	if err != nil {
		return fmt.Errorf("ilink: SendTyping: %w", err)
	}

	s := 0
	if status {
		s = 1
	}
	body := map[string]any{
		"ilink_user_id": userID,
		"typing_ticket": cfg,
		"status":        s,
	}
	var resp struct {
		Ret int `json:"ret"`
	}
	if err := c.postJSON(ctx, "/ilink/bot/sendtyping", body, &resp); err != nil {
		return fmt.Errorf("ilink: SendTyping: %w", err)
	}
	return checkRet("sendtyping", resp.Ret)
}

// ---- internal helpers ----

// getUpdates performs one long-poll call and returns the raw wire messages plus
// the updated cursor.
func (c *Client) getUpdates(ctx context.Context, cursor string) ([]wireMessage, string, error) {
	body := map[string]any{
		"get_updates_buf": cursor,
		"base_info":       map[string]string{"channel_version": channelVersion},
	}
	var resp struct {
		Ret           int           `json:"ret"`
		Msgs          []wireMessage `json:"msgs"`
		GetUpdatesBuf string        `json:"get_updates_buf"`
	}
	if err := c.postJSON(ctx, "/ilink/bot/getupdates", body, &resp); err != nil {
		return nil, "", err
	}
	if err := checkRet("getupdates", resp.Ret); err != nil {
		return nil, "", err
	}
	return resp.Msgs, resp.GetUpdatesBuf, nil
}

// getConfig fetches the typing_ticket for the given user.
func (c *Client) getConfig(ctx context.Context, userID string) (string, error) {
	body := map[string]string{"ilink_user_id": userID}
	var resp struct {
		Ret          int    `json:"ret"`
		TypingTicket string `json:"typing_ticket"`
	}
	if err := c.postJSON(ctx, "/ilink/bot/getconfig", body, &resp); err != nil {
		return "", err
	}
	if err := checkRet("getconfig", resp.Ret); err != nil {
		return "", err
	}
	return resp.TypingTicket, nil
}
