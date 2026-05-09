package ilink

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---- AES-128-ECB round-trip ----

func TestEncryptDecryptECB(t *testing.T) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	cases := [][]byte{
		[]byte("hello"),
		[]byte("exactly16bytess!"),
		make([]byte, 1024),
		{},
	}
	for _, plain := range cases {
		enc, err := encryptECB(key, plain)
		if err != nil {
			t.Fatalf("encryptECB: %v", err)
		}
		got, err := decryptECB(key, enc)
		if err != nil {
			t.Fatalf("decryptECB: %v", err)
		}
		if !bytes.Equal(got, plain) {
			t.Errorf("round-trip failed for %d-byte input", len(plain))
		}
	}
}

func TestDecryptECBRejectsUnaligned(t *testing.T) {
	key := make([]byte, 16)
	_, err := decryptECB(key, []byte("not-aligned")) // 11 bytes
	if err == nil {
		t.Fatal("expected error for non-block-aligned ciphertext")
	}
}

// ---- wire → Message conversion ----

func TestConvertMessageText(t *testing.T) {
	raw := wireMessage{
		FromUserID:   "alice@im.wechat",
		ToUserID:     "bot@im.bot",
		ContextToken: "tok123",
		Seq:          7,
		MessageID:    42,
		SessionID:    "sess-1",
		CreateTimeMs: 1700000000000,
		MessageType:  1,
		ItemList: []wireItem{
			{Type: 1, TextItem: &wireTextItem{Text: "hi"}},
		},
	}
	msg := convertMessage(raw)

	if msg.From != "alice@im.wechat" {
		t.Errorf("From = %q, want %q", msg.From, "alice@im.wechat")
	}
	if msg.Text() != "hi" {
		t.Errorf("Text() = %q, want %q", msg.Text(), "hi")
	}
	if msg.Seq != 7 {
		t.Errorf("Seq = %d, want 7", msg.Seq)
	}
	if msg.MessageID != 42 {
		t.Errorf("MessageID = %d, want 42", msg.MessageID)
	}
	if msg.SessionID != "sess-1" {
		t.Errorf("SessionID = %q", msg.SessionID)
	}
	if msg.CreateTimeMs != 1700000000000 {
		t.Errorf("CreateTimeMs = %d", msg.CreateTimeMs)
	}
	if msg.IsGroup() {
		t.Error("IsGroup() = true, want false")
	}
}

func TestConvertMessageGroup(t *testing.T) {
	raw := wireMessage{
		FromUserID: "alice@im.wechat",
		GroupID:    "grp@im.group",
		ItemList:   []wireItem{{Type: 1, TextItem: &wireTextItem{Text: "hello group"}}},
	}
	msg := convertMessage(raw)
	if !msg.IsGroup() {
		t.Error("IsGroup() = false, want true")
	}
	if msg.GroupID != "grp@im.group" {
		t.Errorf("GroupID = %q", msg.GroupID)
	}
}

func TestConvertMessageMultiText(t *testing.T) {
	raw := wireMessage{
		ItemList: []wireItem{
			{Type: 1, TextItem: &wireTextItem{Text: "foo"}},
			{Type: 1, TextItem: &wireTextItem{Text: "bar"}},
		},
	}
	msg := convertMessage(raw)
	if msg.Text() != "foobar" {
		t.Errorf("Text() = %q, want %q", msg.Text(), "foobar")
	}
}

func TestConvertMessageImage(t *testing.T) {
	raw := wireMessage{
		ItemList: []wireItem{
			{
				Type: int(ItemTypeImage),
				ImageItem: &wireImageItem{
					Media: &wireCDNMedia{
						EncryptQueryParam: "eqp",
						AESKey:            "k64",
						EncryptType:       1,
						FullURL:           "https://cdn.example.com/img",
					},
					ThumbMedia:  &wireCDNMedia{EncryptQueryParam: "thumb-eqp"},
					AESKey:      "hexaeskey",
					URL:         "https://cdn.example.com/raw",
					ThumbWidth:  100,
					ThumbHeight: 80,
					ThumbSize:   2048,
					HDSize:      204800,
					MidSize:     40960,
				},
			},
		},
	}
	msg := convertMessage(raw)
	if len(msg.Items) != 1 || msg.Items[0].Type != ItemTypeImage {
		t.Fatal("expected one image item")
	}
	img := msg.Items[0].Image
	if img == nil {
		t.Fatal("Image is nil")
	}
	if img.Media == nil || img.Media.EncryptQueryParam != "eqp" {
		t.Errorf("Media.EncryptQueryParam = %q", img.Media.EncryptQueryParam)
	}
	if img.Media.FullURL != "https://cdn.example.com/img" {
		t.Errorf("Media.FullURL = %q", img.Media.FullURL)
	}
	if img.ThumbMedia == nil || img.ThumbMedia.EncryptQueryParam != "thumb-eqp" {
		t.Error("ThumbMedia not set")
	}
	if img.AESKey != "hexaeskey" {
		t.Errorf("AESKey = %q", img.AESKey)
	}
	if img.ThumbWidth != 100 || img.ThumbHeight != 80 {
		t.Errorf("thumb dims = %dx%d", img.ThumbWidth, img.ThumbHeight)
	}
	if img.HDSize != 204800 {
		t.Errorf("HDSize = %d", img.HDSize)
	}
}

func TestConvertMessageVoice(t *testing.T) {
	raw := wireMessage{
		ItemList: []wireItem{
			{
				Type: int(ItemTypeVoice),
				VoiceItem: &wireVoiceItem{
					Media:      &wireCDNMedia{EncryptQueryParam: "voice-eqp", AESKey: "vk"},
					EncodeType: int(VoiceEncodeSilk),
					SampleRate: 16000,
					Playtime:   5000,
					Text:       "hello transcript",
				},
			},
		},
	}
	msg := convertMessage(raw)
	v := msg.Items[0].Voice
	if v == nil {
		t.Fatal("Voice is nil")
	}
	if v.EncodeType != VoiceEncodeSilk {
		t.Errorf("EncodeType = %v", v.EncodeType)
	}
	if v.SampleRate != 16000 {
		t.Errorf("SampleRate = %d", v.SampleRate)
	}
	if v.PlaytimeMs != 5000 {
		t.Errorf("PlaytimeMs = %d", v.PlaytimeMs)
	}
	if v.RecognText != "hello transcript" {
		t.Errorf("RecognText = %q", v.RecognText)
	}
}

func TestConvertMessageFile(t *testing.T) {
	raw := wireMessage{
		ItemList: []wireItem{
			{
				Type: int(ItemTypeFile),
				FileItem: &wireFileItem{
					Media:    &wireCDNMedia{EncryptQueryParam: "file-eqp"},
					FileName: "report.pdf",
					MD5:      "deadbeef",
					Len:      "98765",
				},
			},
		},
	}
	msg := convertMessage(raw)
	f := msg.Items[0].File
	if f == nil {
		t.Fatal("File is nil")
	}
	if f.FileName != "report.pdf" {
		t.Errorf("FileName = %q", f.FileName)
	}
	if f.Size != 98765 {
		t.Errorf("Size = %d, want 98765", f.Size)
	}
	if f.MD5 != "deadbeef" {
		t.Errorf("MD5 = %q", f.MD5)
	}
}

func TestConvertMessageVideo(t *testing.T) {
	raw := wireMessage{
		ItemList: []wireItem{
			{
				Type: int(ItemTypeVideo),
				VideoItem: &wireVideoItem{
					Media:      &wireCDNMedia{EncryptQueryParam: "vid-eqp"},
					ThumbMedia: &wireCDNMedia{EncryptQueryParam: "thumb-vid-eqp"},
					VideoSize:  1024000,
					PlayLength: 30000,
					VideoMD5:   "cafebabe",
				},
			},
		},
	}
	msg := convertMessage(raw)
	vid := msg.Items[0].Video
	if vid == nil {
		t.Fatal("Video is nil")
	}
	if vid.PlayLengthMs != 30000 {
		t.Errorf("PlayLengthMs = %d", vid.PlayLengthMs)
	}
	if vid.ThumbMedia == nil || vid.ThumbMedia.EncryptQueryParam != "thumb-vid-eqp" {
		t.Error("ThumbMedia not set")
	}
}

func TestConvertMessageRefMsg(t *testing.T) {
	raw := wireMessage{
		ItemList: []wireItem{
			{
				Type:    int(ItemTypeText),
				TextItem: &wireTextItem{Text: "reply"},
				MsgID:   "msg-001",
				RefMsg: &wireRefMessage{
					Title: "original message",
					MessageItem: &wireItem{
						Type:     int(ItemTypeText),
						TextItem: &wireTextItem{Text: "original"},
					},
				},
				IsCompleted:  true,
				CreateTimeMs: 1700000001000,
			},
		},
	}
	msg := convertMessage(raw)
	item := msg.Items[0]
	if item.MsgID != "msg-001" {
		t.Errorf("MsgID = %q", item.MsgID)
	}
	if !item.IsCompleted {
		t.Error("IsCompleted = false, want true")
	}
	if item.CreateTimeMs != 1700000001000 {
		t.Errorf("CreateTimeMs = %d", item.CreateTimeMs)
	}
	if item.RefMsg == nil {
		t.Fatal("RefMsg is nil")
	}
	if item.RefMsg.Title != "original message" {
		t.Errorf("RefMsg.Title = %q", item.RefMsg.Title)
	}
	if item.RefMsg.Item == nil || item.RefMsg.Item.Text == nil || item.RefMsg.Item.Text.Text != "original" {
		t.Error("RefMsg.Item not converted correctly")
	}
}

// ---- marshalItems round-trip ----

func TestMarshalItemsText(t *testing.T) {
	items := []Item{TextMessage("hello")}
	wired := marshalItems(items)
	if len(wired) != 1 {
		t.Fatalf("len = %d, want 1", len(wired))
	}
	if wired[0].TextItem == nil || wired[0].TextItem.Text != "hello" {
		t.Errorf("unexpected wire item: %+v", wired[0])
	}
}

func TestMarshalItemsImage(t *testing.T) {
	img := &ImageItem{
		Media: &CDNMedia{
			EncryptQueryParam: "eqp",
			AESKey:            "k64",
			EncryptType:       1,
			FullURL:           "https://cdn.example.com/img",
		},
		ThumbMedia: &CDNMedia{EncryptQueryParam: "thumb"},
	}
	items := []Item{{Type: ItemTypeImage, Image: img}}
	wired := marshalItems(items)
	if wired[0].ImageItem == nil {
		t.Fatal("expected image_item")
	}
	if wired[0].ImageItem.Media == nil || wired[0].ImageItem.Media.EncryptQueryParam != "eqp" {
		t.Errorf("media.encrypt_query_param = %q", wired[0].ImageItem.Media.EncryptQueryParam)
	}
	if wired[0].ImageItem.Media.FullURL != "https://cdn.example.com/img" {
		t.Errorf("media.full_url = %q", wired[0].ImageItem.Media.FullURL)
	}
}

func TestMarshalItemsFile(t *testing.T) {
	f := &FileItem{
		Media:    &CDNMedia{EncryptQueryParam: "feqp"},
		FileName: "doc.pdf",
		MD5:      "abc",
		Size:     12345,
	}
	items := []Item{{Type: ItemTypeFile, File: f}}
	wired := marshalItems(items)
	if wired[0].FileItem == nil {
		t.Fatal("expected file_item")
	}
	if wired[0].FileItem.Len != "12345" {
		t.Errorf("Len = %q, want %q", wired[0].FileItem.Len, "12345")
	}
}

// ---- HandlerFunc ----

func TestHandlerFunc(t *testing.T) {
	called := false
	var h Handler = HandlerFunc(func(ctx context.Context, msg *Message) error {
		called = true
		return nil
	})
	if err := h.ServeMessage(context.Background(), &Message{}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("handler was not called")
	}
}

// ---- TextMessage helper ----

func TestTextMessageHelper(t *testing.T) {
	item := TextMessage("world")
	if item.Type != ItemTypeText {
		t.Errorf("Type = %v", item.Type)
	}
	if item.Text == nil || item.Text.Text != "world" {
		t.Errorf("Text = %v", item.Text)
	}
}

// ---- APIError ----

func TestAPIError(t *testing.T) {
	err := checkRet("someop", 42)
	if err == nil {
		t.Fatal("expected error for non-zero ret")
	}
	if err.Error() != "ilink: someop returned ret=42" {
		t.Errorf("unexpected error string: %q", err.Error())
	}
}

func TestCheckRetZero(t *testing.T) {
	if err := checkRet("op", 0); err != nil {
		t.Fatalf("expected nil for ret=0, got %v", err)
	}
}

// ---- sendTyping status values ----

func TestSendTypingStatus(t *testing.T) {
	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getconfig":
			json.NewEncoder(w).Encode(map[string]any{"ret": 0, "typing_ticket": "tkt"})
		case "/ilink/bot/sendtyping":
			json.NewDecoder(r.Body).Decode(&capturedBody)
			json.NewEncoder(w).Encode(map[string]any{"ret": 0})
		}
	}))
	defer srv.Close()

	c := NewClient(Token{BotToken: "test"}, WithBaseURL(srv.URL))

	// status=true → 1 (typing)
	if err := c.SendTyping(context.Background(), "user1@im.wechat", true); err != nil {
		t.Fatalf("SendTyping true: %v", err)
	}
	if v, _ := capturedBody["status"].(float64); v != 1 {
		t.Errorf("status for typing=true: got %v, want 1", capturedBody["status"])
	}

	// status=false → 2 (cancel)
	if err := c.SendTyping(context.Background(), "user1@im.wechat", false); err != nil {
		t.Fatalf("SendTyping false: %v", err)
	}
	if v, _ := capturedBody["status"].(float64); v != 2 {
		t.Errorf("status for typing=false: got %v, want 2", capturedBody["status"])
	}
}

// ---- session timeout ----

func TestGetUpdatesSessionTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"ret":     0,
			"errcode": -14,
			"errmsg":  "session timeout",
		})
	}))
	defer srv.Close()

	c := NewClient(Token{BotToken: "test"}, WithBaseURL(srv.URL))
	_, _, _, err := c.getUpdates(context.Background(), "")
	if err != ErrSessionTimeout {
		t.Errorf("expected ErrSessionTimeout, got %v", err)
	}
}

// ---- RecallMessage ----

func TestRecallMessage(t *testing.T) {
	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		json.NewEncoder(w).Encode(map[string]any{"ret": 0})
	}))
	defer srv.Close()

	c := NewClient(Token{BotToken: "test"}, WithBaseURL(srv.URL))
	if err := c.RecallMessage(context.Background(), 99999); err != nil {
		t.Fatalf("RecallMessage: %v", err)
	}
	if v, _ := capturedBody["msg_id"].(float64); int64(v) != 99999 {
		t.Errorf("msg_id = %v, want 99999", capturedBody["msg_id"])
	}
}
