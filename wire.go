package ilink

// wireMessage is the raw JSON shape returned by /ilink/bot/getupdates.
// It is unexported; callers work with [Message].
type wireMessage struct {
	FromUserID   string        `json:"from_user_id"`
	ToUserID     string        `json:"to_user_id"`
	GroupID      string        `json:"group_id,omitempty"`
	MessageType  int           `json:"message_type"`
	MessageState int           `json:"message_state"`
	ContextToken string        `json:"context_token"`
	ItemList     []wireItem    `json:"item_list"`
}

type wireItem struct {
	Type      int            `json:"type"`
	TextItem  *wireTextItem  `json:"text_item,omitempty"`
	ImageItem *wireMediaItem `json:"image_item,omitempty"`
	VoiceItem *wireVoiceItem `json:"voice_item,omitempty"`
	FileItem  *wireMediaItem `json:"file_item,omitempty"`
	VideoItem *wireVideoItem `json:"video_item,omitempty"`
}

type wireTextItem struct {
	Text string `json:"text"`
}

type wireMediaItem struct {
	FileName          string `json:"file_name,omitempty"`
	FileSize          int64  `json:"file_size"`
	FileMD5           string `json:"file_md5"`
	EncryptedSize     int64  `json:"encrypted_size"`
	AESKey            string `json:"aes_key"`
	EncryptQueryParam string `json:"encrypt_query_param"`
}

type wireVoiceItem struct {
	wireMediaItem
	DurationMs int    `json:"duration_ms"`
	RecognText string `json:"recogn_text,omitempty"`
}

type wireVideoItem struct {
	wireMediaItem
	DurationMs int            `json:"duration_ms"`
	ThumbItem  *wireMediaItem `json:"thumb_item,omitempty"`
}

// convertMessage translates a wireMessage into the public Message type.
// Only inbound messages (MessageType == 1) are delivered to handlers; the
// caller in getUpdates filters for this already, but the conversion is kept
// neutral so tests can exercise it independently.
func convertMessage(w wireMessage) *Message {
	items := make([]Item, 0, len(w.ItemList))
	for _, wi := range w.ItemList {
		items = append(items, convertItem(wi))
	}
	return &Message{
		From:         w.FromUserID,
		To:           w.ToUserID,
		GroupID:      w.GroupID,
		ContextToken: w.ContextToken,
		Items:        items,
	}
}

func convertItem(wi wireItem) Item {
	switch ItemType(wi.Type) {
	case ItemTypeText:
		if wi.TextItem != nil {
			return Item{Type: ItemTypeText, Text: &TextItem{Text: wi.TextItem.Text}}
		}
	case ItemTypeImage:
		if wi.ImageItem != nil {
			m := convertMediaItem(wi.ImageItem)
			return Item{Type: ItemTypeImage, Image: m}
		}
	case ItemTypeVoice:
		if wi.VoiceItem != nil {
			v := &VoiceItem{
				MediaItem:  *convertMediaItem(&wi.VoiceItem.wireMediaItem),
				DurationMs: wi.VoiceItem.DurationMs,
				RecognText: wi.VoiceItem.RecognText,
			}
			return Item{Type: ItemTypeVoice, Voice: v}
		}
	case ItemTypeFile:
		if wi.FileItem != nil {
			return Item{Type: ItemTypeFile, File: convertMediaItem(wi.FileItem)}
		}
	case ItemTypeVideo:
		if wi.VideoItem != nil {
			vid := &VideoItem{
				MediaItem:  *convertMediaItem(&wi.VideoItem.wireMediaItem),
				DurationMs: wi.VideoItem.DurationMs,
			}
			if wi.VideoItem.ThumbItem != nil {
				vid.Thumb = convertMediaItem(wi.VideoItem.ThumbItem)
			}
			return Item{Type: ItemTypeVideo, Video: vid}
		}
	}
	return Item{Type: ItemType(wi.Type)}
}

func convertMediaItem(w *wireMediaItem) *MediaItem {
	return &MediaItem{
		FileName:          w.FileName,
		FileSize:          w.FileSize,
		FileMD5:           w.FileMD5,
		EncryptedSize:     w.EncryptedSize,
		AESKey:            w.AESKey,
		EncryptQueryParam: w.EncryptQueryParam,
	}
}

// ---- outbound wire types ----

type sendMessageRequest struct {
	Msg sendMessageBody `json:"msg"`
}

type sendMessageBody struct {
	ToUserID     string         `json:"to_user_id"`
	GroupID      string         `json:"group_id,omitempty"`
	MessageType  int            `json:"message_type"`
	MessageState int            `json:"message_state"`
	ContextToken string         `json:"context_token"`
	ItemList     []outboundItem `json:"item_list"`
}

type outboundItem struct {
	Type      int            `json:"type"`
	TextItem  *wireTextItem  `json:"text_item,omitempty"`
	ImageItem *wireMediaItem `json:"image_item,omitempty"`
	VoiceItem *wireVoiceItem `json:"voice_item,omitempty"`
	FileItem  *wireMediaItem `json:"file_item,omitempty"`
	VideoItem *wireVideoItem `json:"video_item,omitempty"`
}

// marshalItems converts []Item to []outboundItem for the wire format.
func marshalItems(items []Item) []outboundItem {
	out := make([]outboundItem, 0, len(items))
	for _, item := range items {
		oi := outboundItem{Type: int(item.Type)}
		switch item.Type {
		case ItemTypeText:
			if item.Text != nil {
				oi.TextItem = &wireTextItem{Text: item.Text.Text}
			}
		case ItemTypeImage:
			if item.Image != nil {
				oi.ImageItem = marshalMediaItem(item.Image)
			}
		case ItemTypeVoice:
			if item.Voice != nil {
				oi.VoiceItem = &wireVoiceItem{
					wireMediaItem: *marshalMediaItem(&item.Voice.MediaItem),
					DurationMs:    item.Voice.DurationMs,
				}
			}
		case ItemTypeFile:
			if item.File != nil {
				oi.FileItem = marshalMediaItem(item.File)
			}
		case ItemTypeVideo:
			if item.Video != nil {
				wv := &wireVideoItem{
					wireMediaItem: *marshalMediaItem(&item.Video.MediaItem),
					DurationMs:    item.Video.DurationMs,
				}
				if item.Video.Thumb != nil {
					wv.ThumbItem = marshalMediaItem(item.Video.Thumb)
				}
				oi.VideoItem = wv
			}
		}
		out = append(out, oi)
	}
	return out
}

func marshalMediaItem(m *MediaItem) *wireMediaItem {
	return &wireMediaItem{
		FileName:          m.FileName,
		FileSize:          m.FileSize,
		FileMD5:           m.FileMD5,
		EncryptedSize:     m.EncryptedSize,
		AESKey:            m.AESKey,
		EncryptQueryParam: m.EncryptQueryParam,
	}
}
