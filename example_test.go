package ilink_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/lib-x/ilink"
)

// ExampleStartLogin shows how to perform a QR code login and persist the token.
func ExampleStartLogin() {
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

	// Persist token.BotToken for reuse across restarts.
	data, _ := json.Marshal(token)
	os.WriteFile("token.json", data, 0o600)
}

// ExampleClient_ListenAndServe shows the minimal echo-bot pattern.
func ExampleClient_ListenAndServe() {
	// Load a previously persisted token.
	data, err := os.ReadFile("token.json")
	if err != nil {
		log.Fatal(err)
	}
	var token ilink.Token
	if err := json.Unmarshal(data, &token); err != nil {
		log.Fatal(err)
	}

	client := ilink.NewClient(token)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err = client.ListenAndServe(ctx, ilink.HandlerFunc(func(ctx context.Context, msg *ilink.Message) error {
		log.Printf("message from %s: %s", msg.From, msg.Text())
		return client.Reply(ctx, msg, ilink.TextMessage("pong"))
	}))
	if err != nil {
		log.Fatal(err)
	}
}

// ExampleClient_Send shows how to proactively send a message to a user.
func ExampleClient_Send() {
	client := ilink.NewClient(ilink.Token{BotToken: "…"})

	err := client.Send(context.Background(), &ilink.OutboundMessage{
		To:           "o9cq800kum_xxx@im.wechat",
		ContextToken: "AARzJWAF…", // from a previous inbound message
		Items:        []ilink.Item{ilink.TextMessage("Hello!")},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// ExampleClient_UploadMedia shows how to upload an image and send it.
func ExampleClient_UploadMedia() {
	client := ilink.NewClient(ilink.Token{BotToken: "…"})
	ctx := context.Background()

	data, err := os.ReadFile("photo.jpg")
	if err != nil {
		log.Fatal(err)
	}

	media, err := client.UploadMedia(ctx, data, &ilink.UploadOptions{
		FileName:  "photo.jpg",
		MediaType: ilink.UploadMediaTypeImage,
		ToUserID:  "o9cq800kum_xxx@im.wechat",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Send the uploaded image in a message.
	err = client.Send(ctx, &ilink.OutboundMessage{
		To:           "o9cq800kum_xxx@im.wechat",
		ContextToken: "AARzJWAF…",
		Items: []ilink.Item{
			{
				Type:  ilink.ItemTypeImage,
				Image: &ilink.ImageItem{Media: media},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// ExampleDecryptMedia shows how to decrypt a received media file.
func ExampleDecryptMedia() {
	// Assume msg is an inbound *ilink.Message containing an image.
	var msg *ilink.Message

	for _, item := range msg.Items {
		if item.Type != ilink.ItemTypeImage || item.Image == nil {
			continue
		}
		img := item.Image
		if img.Media == nil {
			continue
		}
		// Use FullURL if available, otherwise construct from EncryptQueryParam.
		cdnURL := img.Media.FullURL
		if cdnURL == "" {
			cdnURL = "https://novac2c.cdn.weixin.qq.com/c2c?" + img.Media.EncryptQueryParam
		}
		plain, err := ilink.DownloadAndDecrypt(context.Background(), cdnURL, img.Media)
		if err != nil {
			log.Fatal(err)
		}
		os.WriteFile("received.jpg", plain, 0o644)
	}
}
