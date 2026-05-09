package ilink

import (
	"bytes"
	"context"
	"crypto/rand"
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

func TestConvertMessageNoText(t *testing.T) {
	raw := wireMessage{
		ItemList: []wireItem{
			{Type: int(ItemTypeImage), ImageItem: &wireMediaItem{FileMD5: "abc"}},
		},
	}
	msg := convertMessage(raw)
	if msg.Text() != "" {
		t.Errorf("Text() = %q, want empty", msg.Text())
	}
	if len(msg.Items) != 1 || msg.Items[0].Type != ItemTypeImage {
		t.Error("expected one image item")
	}
}

// ---- marshalItems round-trip (public → wire → public) ----

func TestMarshalItemsText(t *testing.T) {
	items := []Item{TextMessage("hello")}
	wire := marshalItems(items)
	if len(wire) != 1 {
		t.Fatalf("len = %d, want 1", len(wire))
	}
	if wire[0].TextItem == nil || wire[0].TextItem.Text != "hello" {
		t.Errorf("unexpected wire item: %+v", wire[0])
	}
}

func TestMarshalItemsMedia(t *testing.T) {
	media := &MediaItem{FileMD5: "deadbeef", AESKey: "k", EncryptQueryParam: "q"}
	items := []Item{{Type: ItemTypeImage, Image: media}}
	wire := marshalItems(items)
	if wire[0].ImageItem == nil {
		t.Fatal("expected image_item")
	}
	if wire[0].ImageItem.FileMD5 != "deadbeef" {
		t.Errorf("FileMD5 = %q", wire[0].ImageItem.FileMD5)
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
