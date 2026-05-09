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
	Image *ImageItem
	Voice *VoiceItem
	File  *FileItem
	Video *VideoItem

	// RefMsg is set when this item quotes a previously sent message.
	RefMsg *RefMessage

	// MsgID is the per-item message ID, set on inbound items.
	MsgID string

	// CreateTimeMs is the item creation timestamp in milliseconds (inbound only).
	CreateTimeMs int64
	// UpdateTimeMs is the item last-update timestamp in milliseconds (inbound only).
	UpdateTimeMs int64
	// IsCompleted indicates the item content is fully delivered (inbound only).
	IsCompleted bool
}

// TextMessage returns an [Item] containing a plain text payload.
func TextMessage(text string) Item {
	return Item{Type: ItemTypeText, Text: &TextItem{Text: text}}
}

// TextItem holds a plain-text payload.
type TextItem struct {
	Text string
}

// CDNMedia is a CDN reference to an AES-128-ECB-encrypted media file.
// It is embedded in [ImageItem], [VoiceItem], [FileItem], and [VideoItem].
type CDNMedia struct {
	// EncryptQueryParam is the encrypted CDN parameter string used to
	// download or reference the file.
	EncryptQueryParam string
	// AESKey is the base64-encoded 128-bit AES key.
	AESKey string
	// EncryptType: 0 = encrypt fileid only; 1 = pack thumbnail/mid-size info.
	EncryptType int
	// FullURL is the complete download URL returned by the server (when present).
	// If empty, construct the URL from EncryptQueryParam and the CDN base.
	FullURL string
}

// MediaItem is the legacy unified CDN reference kept for backwards compatibility
// with [Client.UploadMedia]. New inbound messages use the richer per-type structs.
//
// Deprecated: use [CDNMedia] embedded in [ImageItem], [VoiceItem], etc.
type MediaItem = CDNMedia

// ImageItem is an image message with optional thumbnail.
type ImageItem struct {
	// Media is the CDN reference to the full-size image.
	Media *CDNMedia
	// ThumbMedia is the CDN reference to the thumbnail, if present.
	ThumbMedia *CDNMedia

	// AESKey is the raw hex AES-128 key (16 bytes) preferred for inbound
	// decryption over Media.AESKey.
	AESKey string
	// URL is a direct image URL (when provided by the server).
	URL string

	// Dimensions of the thumbnail and high-def variants (pixels / bytes).
	ThumbWidth  int
	ThumbHeight int
	ThumbSize   int
	MidSize     int
	HDSize      int
}

// VoiceEncodeType indicates the audio encoding of a voice message.
type VoiceEncodeType int

const (
	VoiceEncodePCM      VoiceEncodeType = 1
	VoiceEncodeADPCM    VoiceEncodeType = 2
	VoiceEncodeFeature  VoiceEncodeType = 3
	VoiceEncodeSpeex    VoiceEncodeType = 4
	VoiceEncodeAMR      VoiceEncodeType = 5
	VoiceEncodeSilk     VoiceEncodeType = 6
	VoiceEncodeMP3      VoiceEncodeType = 7
	VoiceEncodeOGGSpeex VoiceEncodeType = 8
)

// VoiceItem is a voice message.
type VoiceItem struct {
	// Media is the CDN reference to the voice file.
	Media *CDNMedia

	// EncodeType is the audio encoding; [VoiceEncodeSilk] is the most common.
	EncodeType VoiceEncodeType
	// BitsPerSample and SampleRate describe the audio format.
	BitsPerSample int
	SampleRate    int
	// PlaytimeMs is the audio duration in milliseconds.
	PlaytimeMs int
	// RecognText is the ASR transcript (inbound only, may be empty).
	RecognText string
}

// FileItem is a file attachment.
type FileItem struct {
	// Media is the CDN reference to the file.
	Media *CDNMedia

	// FileName is the original file name.
	FileName string
	// MD5 is the plaintext file MD5 (hex).
	MD5 string
	// Size is the plaintext file size in bytes.
	Size int64
}

// VideoItem is a video message with an optional thumbnail.
type VideoItem struct {
	// Media is the CDN reference to the video.
	Media *CDNMedia
	// ThumbMedia is the CDN reference to the thumbnail.
	ThumbMedia *CDNMedia

	// VideoSize is the plaintext video size in bytes.
	VideoSize int
	// PlayLengthMs is the video duration in milliseconds.
	PlayLengthMs int
	// VideoMD5 is the plaintext video MD5.
	VideoMD5 string

	ThumbSize   int
	ThumbWidth  int
	ThumbHeight int
}

// RefMessage represents a quoted / referenced message.
type RefMessage struct {
	// Item is the referenced message item.
	Item *Item
	// Title is a plaintext summary of the referenced content.
	Title string
}

// Message is an inbound message received from a WeChat user.
type Message struct {
	// Seq is the message sequence number.
	Seq int64
	// MessageID is the unique message ID.
	MessageID int64

	// From is the sender's user ID (format: "xxx@im.wechat").
	From string
	// To is the bot's own user ID (format: "xxx@im.bot").
	To string
	// GroupID is set when the message was sent in a group chat.
	GroupID string
	// SessionID is the conversation session identifier.
	SessionID string

	// CreateTimeMs is the message creation timestamp in milliseconds.
	CreateTimeMs int64
	// UpdateTimeMs is the message last-update timestamp in milliseconds.
	UpdateTimeMs int64

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
