package ilink

// ---- Inbound wire types ----

// wireMessage mirrors proto WeixinMessage from the upstream types.ts.
type wireMessage struct {
	Seq          int64      `json:"seq,omitempty"`
	MessageID    int64      `json:"message_id,omitempty"`
	FromUserID   string     `json:"from_user_id,omitempty"`
	ToUserID     string     `json:"to_user_id,omitempty"`
	ClientID     string     `json:"client_id,omitempty"`
	CreateTimeMs int64      `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64      `json:"update_time_ms,omitempty"`
	DeleteTimeMs int64      `json:"delete_time_ms,omitempty"`
	SessionID    string     `json:"session_id,omitempty"`
	GroupID      string     `json:"group_id,omitempty"`
	MessageType  int        `json:"message_type,omitempty"`
	MessageState int        `json:"message_state,omitempty"`
	ItemList     []wireItem `json:"item_list,omitempty"`
	ContextToken string     `json:"context_token,omitempty"`
}

// wireItem mirrors MessageItem from types.ts.
type wireItem struct {
	Type         int             `json:"type,omitempty"`
	CreateTimeMs int64           `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64           `json:"update_time_ms,omitempty"`
	IsCompleted  bool            `json:"is_completed,omitempty"`
	MsgID        string          `json:"msg_id,omitempty"`
	RefMsg       *wireRefMessage `json:"ref_msg,omitempty"`
	TextItem     *wireTextItem   `json:"text_item,omitempty"`
	ImageItem    *wireImageItem  `json:"image_item,omitempty"`
	VoiceItem    *wireVoiceItem  `json:"voice_item,omitempty"`
	FileItem     *wireFileItem   `json:"file_item,omitempty"`
	VideoItem    *wireVideoItem  `json:"video_item,omitempty"`
}

// wireRefMessage mirrors RefMessage from types.ts.
type wireRefMessage struct {
	MessageItem *wireItem `json:"message_item,omitempty"`
	Title       string    `json:"title,omitempty"`
}

// wireCDNMedia mirrors CDNMedia from types.ts.
type wireCDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AESKey            string `json:"aes_key,omitempty"`
	EncryptType       int    `json:"encrypt_type,omitempty"`
	FullURL           string `json:"full_url,omitempty"`
}

type wireTextItem struct {
	Text string `json:"text,omitempty"`
}

// wireImageItem mirrors ImageItem from types.ts.
type wireImageItem struct {
	Media      *wireCDNMedia `json:"media,omitempty"`
	ThumbMedia *wireCDNMedia `json:"thumb_media,omitempty"`
	AESKey     string        `json:"aeskey,omitempty"` // raw hex, preferred for inbound decrypt
	URL        string        `json:"url,omitempty"`
	MidSize    int           `json:"mid_size,omitempty"`
	ThumbSize  int           `json:"thumb_size,omitempty"`
	ThumbWidth int           `json:"thumb_width,omitempty"`
	ThumbHeight int          `json:"thumb_height,omitempty"`
	HDSize     int           `json:"hd_size,omitempty"`
}

// wireVoiceItem mirrors VoiceItem from types.ts.
type wireVoiceItem struct {
	Media         *wireCDNMedia `json:"media,omitempty"`
	EncodeType    int           `json:"encode_type,omitempty"`
	BitsPerSample int           `json:"bits_per_sample,omitempty"`
	SampleRate    int           `json:"sample_rate,omitempty"`
	Playtime      int           `json:"playtime,omitempty"`
	Text          string        `json:"text,omitempty"` // ASR transcript
}

// wireFileItem mirrors FileItem from types.ts.
type wireFileItem struct {
	Media    *wireCDNMedia `json:"media,omitempty"`
	FileName string        `json:"file_name,omitempty"`
	MD5      string        `json:"md5,omitempty"`
	Len      string        `json:"len,omitempty"` // file size as string
}

// wireVideoItem mirrors VideoItem from types.ts.
type wireVideoItem struct {
	Media       *wireCDNMedia `json:"media,omitempty"`
	VideoSize   int           `json:"video_size,omitempty"`
	PlayLength  int           `json:"play_length,omitempty"`
	VideoMD5   string        `json:"video_md5,omitempty"`
	ThumbMedia  *wireCDNMedia `json:"thumb_media,omitempty"`
	ThumbSize   int           `json:"thumb_size,omitempty"`
	ThumbWidth  int           `json:"thumb_width,omitempty"`
	ThumbHeight int           `json:"thumb_height,omitempty"`
}

// ---- Conversion: wire → public ----

func convertMessage(w wireMessage) *Message {
	items := make([]Item, 0, len(w.ItemList))
	for _, wi := range w.ItemList {
		items = append(items, convertItem(wi))
	}
	return &Message{
		Seq:          w.Seq,
		MessageID:    w.MessageID,
		From:         w.FromUserID,
		To:           w.ToUserID,
		GroupID:      w.GroupID,
		SessionID:    w.SessionID,
		CreateTimeMs: w.CreateTimeMs,
		UpdateTimeMs: w.UpdateTimeMs,
		ContextToken: w.ContextToken,
		Items:        items,
	}
}

func convertItem(wi wireItem) Item {
	item := Item{
		Type:         ItemType(wi.Type),
		MsgID:        wi.MsgID,
		CreateTimeMs: wi.CreateTimeMs,
		UpdateTimeMs: wi.UpdateTimeMs,
		IsCompleted:  wi.IsCompleted,
	}
	if wi.RefMsg != nil {
		ref := &RefMessage{Title: wi.RefMsg.Title}
		if wi.RefMsg.MessageItem != nil {
			sub := convertItem(*wi.RefMsg.MessageItem)
			ref.Item = &sub
		}
		item.RefMsg = ref
	}
	switch ItemType(wi.Type) {
	case ItemTypeText:
		if wi.TextItem != nil {
			item.Text = &TextItem{Text: wi.TextItem.Text}
		}
	case ItemTypeImage:
		if wi.ImageItem != nil {
			item.Image = convertImageItem(wi.ImageItem)
		}
	case ItemTypeVoice:
		if wi.VoiceItem != nil {
			item.Voice = convertVoiceItem(wi.VoiceItem)
		}
	case ItemTypeFile:
		if wi.FileItem != nil {
			item.File = convertFileItem(wi.FileItem)
		}
	case ItemTypeVideo:
		if wi.VideoItem != nil {
			item.Video = convertVideoItem(wi.VideoItem)
		}
	}
	return item
}

func convertCDNMedia(w *wireCDNMedia) *CDNMedia {
	if w == nil {
		return nil
	}
	return &CDNMedia{
		EncryptQueryParam: w.EncryptQueryParam,
		AESKey:            w.AESKey,
		EncryptType:       w.EncryptType,
		FullURL:           w.FullURL,
	}
}

func convertImageItem(w *wireImageItem) *ImageItem {
	return &ImageItem{
		Media:       convertCDNMedia(w.Media),
		ThumbMedia:  convertCDNMedia(w.ThumbMedia),
		AESKey:      w.AESKey,
		URL:         w.URL,
		MidSize:     w.MidSize,
		ThumbSize:   w.ThumbSize,
		ThumbWidth:  w.ThumbWidth,
		ThumbHeight: w.ThumbHeight,
		HDSize:      w.HDSize,
	}
}

func convertVoiceItem(w *wireVoiceItem) *VoiceItem {
	return &VoiceItem{
		Media:         convertCDNMedia(w.Media),
		EncodeType:    VoiceEncodeType(w.EncodeType),
		BitsPerSample: w.BitsPerSample,
		SampleRate:    w.SampleRate,
		PlaytimeMs:    w.Playtime,
		RecognText:    w.Text,
	}
}

func convertFileItem(w *wireFileItem) *FileItem {
	var size int64
	if w.Len != "" {
		size, _ = parseInt64(w.Len)
	}
	return &FileItem{
		Media:    convertCDNMedia(w.Media),
		FileName: w.FileName,
		MD5:      w.MD5,
		Size:     size,
	}
}

func convertVideoItem(w *wireVideoItem) *VideoItem {
	return &VideoItem{
		Media:        convertCDNMedia(w.Media),
		ThumbMedia:   convertCDNMedia(w.ThumbMedia),
		VideoSize:    w.VideoSize,
		PlayLengthMs: w.PlayLength,
		VideoMD5:     w.VideoMD5,
		ThumbSize:    w.ThumbSize,
		ThumbWidth:   w.ThumbWidth,
		ThumbHeight:  w.ThumbHeight,
	}
}

// ---- Outbound wire types ----

type sendMessageRequest struct {
	Msg sendMessageBody `json:"msg"`
}

type sendMessageBody struct {
	ToUserID     string         `json:"to_user_id"`
	GroupID      string         `json:"group_id,omitempty"`
	MessageType  int            `json:"message_type"`
	MessageState int            `json:"message_state"`
	ContextToken string         `json:"context_token,omitempty"`
	ItemList     []outboundItem `json:"item_list"`
}

type outboundItem struct {
	Type      int            `json:"type"`
	TextItem  *wireTextItem  `json:"text_item,omitempty"`
	ImageItem *wireImageItem `json:"image_item,omitempty"`
	VoiceItem *wireVoiceItem `json:"voice_item,omitempty"`
	FileItem  *wireFileItem  `json:"file_item,omitempty"`
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
				oi.ImageItem = marshalImageItem(item.Image)
			}
		case ItemTypeVoice:
			if item.Voice != nil {
				oi.VoiceItem = marshalVoiceItem(item.Voice)
			}
		case ItemTypeFile:
			if item.File != nil {
				oi.FileItem = marshalFileItem(item.File)
			}
		case ItemTypeVideo:
			if item.Video != nil {
				oi.VideoItem = marshalVideoItem(item.Video)
			}
		}
		out = append(out, oi)
	}
	return out
}

func marshalCDNMedia(m *CDNMedia) *wireCDNMedia {
	if m == nil {
		return nil
	}
	return &wireCDNMedia{
		EncryptQueryParam: m.EncryptQueryParam,
		AESKey:            m.AESKey,
		EncryptType:       m.EncryptType,
		FullURL:           m.FullURL,
	}
}

func marshalImageItem(img *ImageItem) *wireImageItem {
	return &wireImageItem{
		Media:      marshalCDNMedia(img.Media),
		ThumbMedia: marshalCDNMedia(img.ThumbMedia),
		AESKey:     img.AESKey,
		URL:        img.URL,
	}
}

func marshalVoiceItem(v *VoiceItem) *wireVoiceItem {
	return &wireVoiceItem{
		Media:         marshalCDNMedia(v.Media),
		EncodeType:    int(v.EncodeType),
		BitsPerSample: v.BitsPerSample,
		SampleRate:    v.SampleRate,
		Playtime:      v.PlaytimeMs,
	}
}

func marshalFileItem(f *FileItem) *wireFileItem {
	return &wireFileItem{
		Media:    marshalCDNMedia(f.Media),
		FileName: f.FileName,
		MD5:      f.MD5,
		Len:      itoa64(f.Size),
	}
}

func marshalVideoItem(v *VideoItem) *wireVideoItem {
	return &wireVideoItem{
		Media:      marshalCDNMedia(v.Media),
		ThumbMedia: marshalCDNMedia(v.ThumbMedia),
		VideoSize:  v.VideoSize,
		PlayLength: v.PlayLengthMs,
		VideoMD5:   v.VideoMD5,
		ThumbSize:  v.ThumbSize,
	}
}
