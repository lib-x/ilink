package ilink

import "context"

// ItemType identifies the kind of content in an [Item].
type ItemType int

const (
	ItemTypeText  ItemType = 1
	ItemTypeImage ItemType = 2
	ItemTypeVoice ItemType = 3
	ItemTypeFile  ItemType = 4
	ItemTypeVideo ItemType = 5
)

// Item is a single content element inside a message.
// Exactly one of the item fields will be non-nil, determined by Type.
type Item struct {
	Type  ItemType
	Text  *TextItem
	Image *MediaItem
	Voice *VoiceItem
	File  *MediaItem
	Video *VideoItem
}

// TextMessage returns an [Item] containing a plain text payload.
func TextMessage(text string) Item {
	return Item{Type: ItemTypeText, Text: &TextItem{Text: text}}
}

// TextItem holds a plain-text payload.
type TextItem struct {
	Text string `json:"text"`
}

// MediaItem describes a CDN-hosted, AES-128-ECB-encrypted media file.
// Use [Client.UploadMedia] to produce one ready for sending.
type MediaItem struct {
	// FileName is the original file name, used for file attachments.
	FileName string `json:"file_name,omitempty"`

	FileSize      int64  `json:"file_size"`
	FileMD5       string `json:"file_md5"`
	EncryptedSize int64  `json:"encrypted_size"`

	// AESKey is the base64-encoded 128-bit AES key used to encrypt the file.
	AESKey string `json:"aes_key"`

	// EncryptQueryParam is the CDN query string returned by getuploadurl,
	// required when referencing this media in a message.
	EncryptQueryParam string `json:"encrypt_query_param"`
}

// VoiceItem is a voice message in Silk encoding, optionally with a transcript.
type VoiceItem struct {
	MediaItem
	DurationMs  int    `json:"duration_ms"`
	RecognText  string `json:"recogn_text,omitempty"` // ASR transcript, inbound only
}

// VideoItem is a video message with an optional thumbnail.
type VideoItem struct {
	MediaItem
	DurationMs int        `json:"duration_ms"`
	Thumb      *MediaItem `json:"thumb_item,omitempty"`
}

// Message is an inbound message received from a WeChat user.
type Message struct {
	// From is the sender's user ID (format: "xxx@im.wechat").
	From string
	// To is the bot's own user ID (format: "xxx@im.bot").
	To string
	// GroupID is set when the message was sent in a group chat.
	GroupID string
	// ContextToken must be echoed back verbatim when replying, so WeChat
	// associates the reply with the correct conversation window.
	ContextToken string
	// Items holds the message content elements in order.
	Items []Item
}

// Text returns the concatenated text of all [ItemTypeText] items in the message,
// or an empty string if there are none.
func (m *Message) Text() string {
	var s string
	for _, item := range m.Items {
		if item.Type == ItemTypeText && item.Text != nil {
			s += item.Text.Text
		}
	}
	return s
}

// IsGroup reports whether this message originated in a group chat.
func (m *Message) IsGroup() bool { return m.GroupID != "" }

// Handler is implemented by any value that can handle an inbound [Message].
// It mirrors the net/http.Handler contract: return nil on success; return a
// non-nil error to signal that processing failed (the polling loop logs the
// error and continues).
type Handler interface {
	ServeMessage(ctx context.Context, msg *Message) error
}

// HandlerFunc is an adapter to allow the use of ordinary functions as
// [Handler]s, analogous to net/http.HandlerFunc.
type HandlerFunc func(ctx context.Context, msg *Message) error

// ServeMessage calls f(ctx, msg).
func (f HandlerFunc) ServeMessage(ctx context.Context, msg *Message) error {
	return f(ctx, msg)
}

// OutboundMessage is a message to be sent to a WeChat user.
type OutboundMessage struct {
	// To is the recipient user ID (format: "xxx@im.wechat").
	To string
	// GroupID optionally targets a group chat.
	GroupID string
	// ContextToken must match the context_token from the inbound message
	// that triggered this reply, so WeChat routes it to the right chat window.
	// Use [Client.Reply] to populate this automatically.
	ContextToken string
	// Items contains the content to send. At least one item is required.
	Items []Item
}
