package ilink

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
)

// postJSON encodes body as JSON, POSTs to base+path with iLink auth headers,
// and decodes the JSON response into dst.
func (c *Client) postJSON(ctx context.Context, path string, body, dst any) error {
	return doJSON(ctx, c.http, c.token.BotToken, c.base, http.MethodPost, path, body, dst)
}

// getJSON performs an unauthenticated GET (used during login before a token
// exists). Pass an empty botToken to skip the Authorization header.
func getJSON(ctx context.Context, base, path string, body, dst any) error {
	return doJSON(ctx, http.DefaultClient, "", base, http.MethodGet, path, body, dst)
}

func doJSON(ctx context.Context, hc *http.Client, botToken, base, method, path string, body, dst any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("ilink: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, base+path, bodyReader)
	if err != nil {
		return fmt.Errorf("ilink: new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// X-WECHAT-UIN: base64(string(randomUint32)) — changes every request as
	// an anti-replay measure.
	uin := base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(uint64(rand.Uint32()), 10)))
	req.Header.Set("X-WECHAT-UIN", uin)

	if botToken != "" {
		req.Header.Set("AuthorizationType", "ilink_bot_token")
		req.Header.Set("Authorization", "Bearer "+botToken)
	}

	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("ilink: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ilink: %s %s: unexpected status %s", method, path, resp.Status)
	}

	if dst != nil {
		if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
			return fmt.Errorf("ilink: %s %s: decode response: %w", method, path, err)
		}
	}
	return nil
}

// putBytes uploads raw bytes to the given CDN URL via HTTP PUT.
func putBytes(ctx context.Context, url string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("ilink: put: %w", err)
	}
	req.ContentLength = int64(len(data))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ilink: put %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("ilink: put %s: unexpected status %s", url, resp.Status)
	}
	return nil
}
