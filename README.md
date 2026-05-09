# ilink

[![Go Reference](https://pkg.go.dev/badge/github.com/lib-x/ilink.svg)](https://pkg.go.dev/github.com/lib-x/ilink)
[![Go Report Card](https://goreportcard.com/badge/github.com/lib-x/ilink)](https://goreportcard.com/report/github.com/lib-x/ilink)

Go SDK for the [Tencent WeChat iLink Bot API](https://docs.openclaw.ai/channels/wechat) — the official, legally-backed WeChat personal account bot interface released in 2026 through the OpenClaw platform.

## Overview

iLink (`ilinkai.weixin.qq.com`) is Tencent's official HTTP/JSON API that lets developers receive and send WeChat messages from a personal account. This package provides an idiomatic Go client for it.

```
WeChat User ──▶ iLink API ──▶ Client.ListenAndServe ──▶ your Handler
your Handler ──▶ Client.Reply ──▶ iLink API ──▶ WeChat User
```

## Requirements

- Go 1.22 or later
- A WeChat account eligible for the iLink Bot feature (via the OpenClaw platform)

## Installation

```bash
go get github.com/lib-x/ilink
```

## Quick start

### 1. Log in

Scan a QR code once to obtain a `Token`. Persist it so you don't need to scan again on restart.

```go
ctx := context.Background()

login, err := ilink.StartLogin(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Println("Scan this URL with WeChat:", login.URL)

token, err := login.Wait(ctx)
if err != nil {
    log.Fatal(err)
}

// Save token.BotToken to disk for reuse.
data, _ := json.Marshal(token)
os.WriteFile("token.json", data, 0600)
```

### 2. Receive and reply to messages

```go
client := ilink.NewClient(token)

err := client.ListenAndServe(ctx, ilink.HandlerFunc(func(ctx context.Context, msg *ilink.Message) error {
    log.Printf("message from %s: %s", msg.From, msg.Text())
    return client.Reply(ctx, msg, ilink.TextMessage("pong"))
}))
```

`ListenAndServe` blocks until `ctx` is cancelled. Use `signal.NotifyContext` for graceful shutdown.

### 3. Send a message

```go
err := client.Send(ctx, &ilink.OutboundMessage{
    To:           "o9cq800kum_xxx@im.wechat",
    ContextToken: msg.ContextToken, // required for correct routing
    Items:        []ilink.Item{ilink.TextMessage("Hello!")},
})
```

### 4. Send an image

```go
data, _ := os.ReadFile("photo.jpg")

media, err := client.UploadMedia(ctx, data, &ilink.UploadOptions{FileName: "photo.jpg"})
if err != nil {
    log.Fatal(err)
}

err = client.Reply(ctx, msg, ilink.Item{Type: ilink.ItemTypeImage, Image: media})
```

Media files are AES-128-ECB encrypted before upload to Tencent CDN. `UploadMedia` handles key generation, encryption, and the CDN PUT automatically.

## API reference

### Authentication

| Symbol | Description |
|--------|-------------|
| `StartLogin(ctx)` | Begin a QR code login session |
| `Login.URL` | Pre-rendered QR code image URL |
| `Login.QRCode` | Raw QR code string (encode yourself if needed) |
| `Login.Wait(ctx)` | Block until the user scans; returns a `Token` |
| `Token` | Persistent credentials — save `BotToken` to disk |

### Client

| Symbol | Description |
|--------|-------------|
| `NewClient(token, ...Option)` | Create a client from a saved `Token` |
| `WithHTTPClient(hc)` | Replace the default `http.Client` |
| `WithLogger(l)` | Set a `log/slog.Logger` for polling errors |
| `WithBaseURL(u)` | Override the API base URL (useful for tests) |

### Messaging

| Symbol | Description |
|--------|-------------|
| `Client.ListenAndServe(ctx, Handler)` | Long-poll loop; calls `Handler` per message |
| `Client.Send(ctx, *OutboundMessage)` | Send a message to any user |
| `Client.Reply(ctx, *Message, ...Item)` | Reply to an inbound message (copies `ContextToken`) |
| `Client.SendTyping(ctx, userID, bool)` | Show/hide the "typing…" indicator |
| `TextMessage(text)` | Construct a plain-text `Item` |

### Media

| Symbol | Description |
|--------|-------------|
| `Client.UploadMedia(ctx, data, *UploadOptions)` | Encrypt and upload; returns a `*MediaItem` |
| `DecryptMedia(item, ciphertext)` | Decrypt a CDN-downloaded blob |
| `DownloadAndDecrypt(ctx, cdnURL, item)` | Fetch and decrypt in one call |

### Handler interface

```go
type Handler interface {
    ServeMessage(ctx context.Context, msg *Message) error
}

// HandlerFunc adapts an ordinary function to Handler, like net/http.HandlerFunc.
type HandlerFunc func(ctx context.Context, msg *Message) error
```

### Message types

| `ItemType` | Constant | Populated field |
|-----------|----------|-----------------|
| Text | `ItemTypeText` | `Item.Text` |
| Image | `ItemTypeImage` | `Item.Image` |
| Voice | `ItemTypeVoice` | `Item.Voice` (includes ASR transcript) |
| File | `ItemTypeFile` | `Item.File` |
| Video | `ItemTypeVideo` | `Item.Video` |

### Errors

| Symbol | Meaning |
|--------|---------|
| `ErrQRCodeExpired` | QR code expired before the user scanned it |
| `ErrQRCodeCancelled` | User cancelled the scan on the phone |
| `ErrLoginTimeout` | `ctx` deadline exceeded during `Login.Wait` |
| `*APIError` | Non-zero `ret` code returned by the iLink API |

## Design notes

The package follows standard library conventions:

- **`Handler` / `HandlerFunc`** mirror `net/http.Handler` / `HandlerFunc`.
- **`ListenAndServe`** blocks and returns on `ctx` cancellation, matching the `net/http` pattern.
- **Functional options** (`WithLogger`, `WithHTTPClient`, …) keep `NewClient` forward-compatible.
- **`context.Context`** is the first argument of every I/O call; all network operations are cancellable.
- **Wire types are unexported.** Callers only interact with `Message`, `Item`, and `MediaItem`; internal JSON shapes live in `wire.go`.

## Legal

iLink is an official Tencent product governed by the *WeChat ClawBot Feature Terms of Service*. Tencent acts solely as a message conduit and does not store message content. Refer to the terms for permitted use cases and prohibited actions.

This SDK is an independent open-source client and is not affiliated with or endorsed by Tencent.

## Keeping the SDK up to date

When `@tencent-weixin/openclaw-weixin` publishes a new version, follow the step-by-step process in [`AGENTS.md`](AGENTS.md) — written for both human maintainers and AI agents.

## License

MIT
