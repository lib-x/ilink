package ilink

import (
	"context"
	"fmt"
	"time"
)

// Token holds the credentials obtained after a successful QR code login.
// Persist this value; supply it to [NewClient] to avoid re-scanning on restart.
type Token struct {
	// BotToken is the Bearer token used to authenticate all API requests.
	BotToken string `json:"bot_token"`
	// BaseURL is the API base URL returned by the server for this account.
	// If empty, the global default is used.
	BaseURL string `json:"baseurl,omitempty"`
}

// Login represents an in-progress QR code login session.
// Call [StartLogin] to begin, then [Login.Wait] to block until the user scans.
type Login struct {
	// QRCode is the raw QR code string that can be encoded as an image.
	QRCode string
	// URL is a pre-rendered QR code image URL ready for display in a browser.
	URL string

	qrcode string // same as QRCode, kept for internal polling
	base   string // API base to use for polling
}

// StartLogin initiates a new QR code login session and returns a [Login] value.
// Display Login.URL or encode Login.QRCode as an image, then call [Login.Wait].
func StartLogin(ctx context.Context) (*Login, error) {
	var resp struct {
		Ret             int    `json:"ret"`
		QRCode          string `json:"qrcode"`
		QRCodeImgURL    string `json:"qrcode_img_content"`
	}
	if err := getJSON(ctx, defaultBase, "/ilink/bot/get_bot_qrcode?bot_type=3", nil, &resp); err != nil {
		return nil, fmt.Errorf("ilink: StartLogin: %w", err)
	}
	if err := checkRet("get_bot_qrcode", resp.Ret); err != nil {
		return nil, err
	}
	return &Login{
		QRCode: resp.QRCode,
		URL:    resp.QRCodeImgURL,
		qrcode: resp.QRCode,
		base:   defaultBase,
	}, nil
}

// Wait polls until the user scans and confirms the QR code, then returns a
// [Token]. It respects ctx cancellation and the QR code's own expiry.
//
// Typical polling cadence is one request per second; the server responds
// immediately when the status changes.
func (l *Login) Wait(ctx context.Context) (*Token, error) {
	url := "/ilink/bot/get_qrcode_status?qrcode=" + l.qrcode

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ErrLoginTimeout
		case <-ticker.C:
		}

		var resp struct {
			Ret      int    `json:"ret"`
			Status   string `json:"status"`
			BotToken string `json:"bot_token"`
			BaseURL  string `json:"baseurl"`
		}
		if err := getJSON(ctx, l.base, url, nil, &resp); err != nil {
			// Transient network error; keep trying.
			continue
		}
		if err := checkRet("get_qrcode_status", resp.Ret); err != nil {
			return nil, err
		}

		switch resp.Status {
		case "confirmed":
			return &Token{BotToken: resp.BotToken, BaseURL: resp.BaseURL}, nil
		case "expired":
			return nil, ErrQRCodeExpired
		case "cancelled":
			return nil, ErrQRCodeCancelled
		}
		// "pending" or "scanned" → keep waiting
	}
}
