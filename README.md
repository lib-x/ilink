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

```
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

`ListenAndServe` blocks until `ctx` is cancelled or the server signals a session timeout ([`ErrSessionTimeout`](#errors)). Use `signal.NotifyContext` for graceful shutdown.

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

media, err := client.UploadMedia(ctx, data, &ilink.UploadOptions{
    FileName:  "photo.jpg",
    MediaType: ilink.UploadMediaTypeImage,
    ToUserID:  msg.From,
})
if err != nil {
    log.Fatal(err)
}

err = client.Reply(ctx, msg, ilink.Item{
    Type:  ilink.ItemTypeImage,
    Image: &ilink.ImageItem{Media: media},
})
```

Media files are AES-128-ECB encrypted before upload to Tencent CDN. `UploadMedia` handles key generation, encryption, and the CDN PUT automatically.

### 5. Download and decrypt a received image

```go
for _, item := range msg.Items {
    if item.Type != ilink.ItemTypeImage || item.Image == nil {
        continue
    }
    img := item.Image
    cdnURL := img.Media.FullURL
    if cdnURL == "" {
        cdnURL = "https://novac2c.cdn.weixin.qq.com/c2c?" + img.Media.EncryptQueryParam
    }
    plain, err := ilink.DownloadAndDecrypt(ctx, cdnURL, img.Media)
    if err != nil {
        log.Fatal(err)
    }
    os.WriteFile("received.jpg", plain, 0644)
}
```

### 6. Recall a message

```go
err := client.RecallMessage(ctx, sentMsgID)
```

### 7. Handle session timeout

When the bot session expires (server `errcode=-14`), `ListenAndServe` returns `ErrSessionTimeout`. Re-scan the QR code to recover:

```go
err := client.ListenAndServe(ctx, handler)
if errors.Is(err, ilink.ErrSessionTimeout) {
    log.Println("session expired — please re-login")
}
```

## API reference

### Authentication

| Symbol | Description |
|---|---|
| `StartLogin(ctx)` | Begin a QR code login session |
| `Login.URL` | Pre-rendered QR code image URL |
| `Login.QRCode` | Raw QR code string (encode yourself if needed) |
| `Login.Wait(ctx)` | Block until the user scans; returns a `Token` |
| `Token` | Persistent credentials — save `BotToken` to disk |

### Client

| Symbol | Description |
|---|---|
| `NewClient(token, ...Option)` | Create a client from a saved `Token` |
| `WithHTTPClient(hc)` | Replace the default `http.Client` |
| `WithLogger(l)` | Set a `log/slog.Logger` for polling errors |
| `WithBaseURL(u)` | Override the API base URL (useful for tests) |

### Messaging

| Symbol | Description |
|---|---|
| `Client.ListenAndServe(ctx, Handler)` | Long-poll loop; calls `Handler` per message |
| `Client.Send(ctx, *OutboundMessage)` | Send a message to any user |
| `Client.Reply(ctx, *Message, ...Item)` | Reply to an inbound message (copies `ContextToken`) |
| `Client.SendTyping(ctx, userID, bool)` | Show/hide the "typing…" indicator |
| `Client.RecallMessage(ctx, msgID)` | Withdraw a previously sent message |
| `TextMessage(text)` | Construct a plain-text `Item` |

### Media

| Symbol | Description |
|---|---|
| `Client.UploadMedia(ctx, data, *UploadOptions)` | Encrypt and upload; returns a `*CDNMedia` |
| `DecryptMedia(item, ciphertext)` | Decrypt a CDN-downloaded blob |
| `DownloadAndDecrypt(ctx, cdnURL, item)` | Fetch and decrypt in one call |

#### UploadOptions

| Field | Description |
|---|---|
| `FileName` | Original file name (embedded in CDN reference) |
| `MediaType` | `UploadMediaTypeImage` / `Video` / `File` / `Voice` |
| `ToUserID` | Target recipient's user ID (required by the server) |
| `ThumbData` | Raw thumbnail bytes for IMAGE / VIDEO uploads |
| `NoThumb` | Suppress thumbnail CDN slot allocation |

### Handler interface

```go
type Handler interface {
    ServeMessage(ctx context.Context, msg *Message) error
}

// HandlerFunc adapts an ordinary function to Handler, like net/http.HandlerFunc.
type HandlerFunc func(ctx context.Context, msg *Message) error
```

### Message

| Field | Type | Description |
|---|---|---|
| `Seq` | `int64` | Message sequence number |
| `MessageID` | `int64` | Unique message ID |
| `From` | `string` | Sender user ID (`xxx@im.wechat`) |
| `To` | `string` | Bot's own user ID (`xxx@im.bot`) |
| `GroupID` | `string` | Set for group chat messages |
| `SessionID` | `string` | Conversation session identifier |
| `CreateTimeMs` | `int64` | Creation timestamp (milliseconds) |
| `UpdateTimeMs` | `int64` | Last-update timestamp (milliseconds) |
| `ContextToken` | `string` | Must be echoed back when replying |
| `Items` | `[]Item` | Message content elements |

### Message types

| `ItemType` | Constant | Populated field |
|---|---|---|
| Text | `ItemTypeText` | `Item.Text *TextItem` |
| Image | `ItemTypeImage` | `Item.Image *ImageItem` |
| Voice | `ItemTypeVoice` | `Item.Voice *VoiceItem` |
| File | `ItemTypeFile` | `Item.File *FileItem` |
| Video | `ItemTypeVideo` | `Item.Video *VideoItem` |

Each `Item` also carries `MsgID`, `CreateTimeMs`, `UpdateTimeMs`, `IsCompleted`, and `RefMsg *RefMessage` (for quoted messages).

### CDNMedia

All media types reference CDN-hosted, AES-128-ECB-encrypted content via `*CDNMedia`:

| Field | Description |
|---|---|
| `EncryptQueryParam` | Encrypted CDN parameter string for download/upload |
| `AESKey` | Base64-encoded 128-bit AES key |
| `EncryptType` | `0` = encrypt fileid only; `1` = pack thumbnail info |
| `FullURL` | Complete download URL when provided by the server |

### Media item types

**ImageItem**

| Field | Description |
|---|---|
| `Media *CDNMedia` | Full-size image CDN reference |
| `ThumbMedia *CDNMedia` | Thumbnail CDN reference |
| `AESKey string` | Raw hex AES-128 key (preferred for inbound decryption) |
| `URL string` | Direct image URL (when provided) |
| `ThumbWidth/Height/Size` | Thumbnail dimensions and byte size |
| `MidSize`, `HDSize` | Mid-resolution and HD byte sizes |

**VoiceItem**

| Field | Description |
|---|---|
| `Media *CDNMedia` | Voice file CDN reference |
| `EncodeType VoiceEncodeType` | Audio encoding (e.g. `VoiceEncodeSilk`) |
| `SampleRate int` | Sample rate in Hz |
| `PlaytimeMs int` | Duration in milliseconds |
| `RecognText string` | ASR transcript (inbound only, may be empty) |

**FileItem**

| Field | Description |
|---|---|
| `Media *CDNMedia` | File CDN reference |
| `FileName string` | Original file name |
| `MD5 string` | Plaintext file MD5 (hex) |
| `Size int64` | Plaintext file size in bytes |

**VideoItem**

| Field | Description |
|---|---|
| `Media *CDNMedia` | Video CDN reference |
| `ThumbMedia *CDNMedia` | Thumbnail CDN reference |
| `PlayLengthMs int` | Duration in milliseconds |
| `VideoSize int` | Plaintext video size in bytes |
| `VideoMD5 string` | Plaintext video MD5 |

### Errors

| Symbol | Meaning |
|---|---|
| `ErrQRCodeExpired` | QR code expired before the user scanned it |
| `ErrQRCodeCancelled` | User cancelled the scan on the phone |
| `ErrLoginTimeout` | `ctx` deadline exceeded during `Login.Wait` |
| `ErrSessionTimeout` | Server `errcode=-14`: bot session expired, re-login required |
| `*APIError` | Non-zero `ret` code returned by the iLink API |

## Design notes

The package follows standard library conventions:

- **`Handler` / `HandlerFunc`** mirror `net/http.Handler` / `HandlerFunc`.
- **`ListenAndServe`** blocks and returns on `ctx` cancellation or `ErrSessionTimeout`, matching the `net/http` pattern.
- **Functional options** (`WithLogger`, `WithHTTPClient`, …) keep `NewClient` forward-compatible.
- **`context.Context`** is the first argument of every I/O call; all network operations are cancellable.
- **Wire types are unexported.** Callers only interact with `Message`, `Item`, `CDNMedia`, and the per-type media structs; internal JSON shapes live in `wire.go`.
- **Dynamic long-poll timeout.** The server's `longpolling_timeout_ms` hint is respected automatically.

## Legal

iLink is an official Tencent product governed by the *WeChat ClawBot Feature Terms of Service*. Tencent acts solely as a message conduit and does not store message content. Refer to the terms for permitted use cases and prohibited actions.

This SDK is an independent open-source client and is not affiliated with or endorsed by Tencent.

## Keeping the SDK up to date

When `@tencent-weixin/openclaw-weixin` publishes a new version, follow the step-by-step process in [`AGENTS.md`](AGENTS.md) — written for both human maintainers and AI agents.

## License

MIT
