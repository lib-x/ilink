package ilink_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/lib-x/ilink"
)

// ExampleStartLogin shows the QR code login flow and how to persist the token.
func ExampleStartLogin() {
	ctx := context.Background()

	login, err := ilink.StartLogin(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Display the URL or encode login.QRCode as an image yourself.
	fmt.Println("scan:", login.URL)

	token, err := login.Wait(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Persist token for reuse across restarts.
	data, _ := json.Marshal(token)
	os.WriteFile("token.json", data, 0600)

	_ = token
}

// ExampleClient_ListenAndServe shows the minimal message-loop setup.
func ExampleClient_ListenAndServe() {
	token := ilink.Token{BotToken: "your-bot-token"}
	client := ilink.NewClient(token)

	ctx := context.Background()
	err := client.ListenAndServe(ctx, ilink.HandlerFunc(func(ctx context.Context, msg *ilink.Message) error {
		return client.Reply(ctx, msg, ilink.TextMessage("pong"))
	}))
	if err != nil {
		log.Fatal(err)
	}
}

// ExampleClient_Reply shows how to reply to an inbound message.
func ExampleClient_Reply() {
	token := ilink.Token{BotToken: "your-bot-token"}
	client := ilink.NewClient(token)

	handler := ilink.HandlerFunc(func(ctx context.Context, msg *ilink.Message) error {
		// Reply echoes context_token automatically.
		return client.Reply(ctx, msg, ilink.TextMessage("你好！"))
	})

	_ = handler
}

// ExampleClient_Send shows how to send an outbound message directly.
func ExampleClient_Send() {
	token := ilink.Token{BotToken: "your-bot-token"}
	client := ilink.NewClient(token)

	ctx := context.Background()
	err := client.Send(ctx, &ilink.OutboundMessage{
		To:           "o9cq800kum_xxx@im.wechat",
		ContextToken: "<context_token from inbound message>",
		Items:        []ilink.Item{ilink.TextMessage("Hello!")},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// ExampleClient_UploadMedia shows how to send an image.
func ExampleClient_UploadMedia() {
	token := ilink.Token{BotToken: "your-bot-token"}
	client := ilink.NewClient(token,
		ilink.WithLogger(slog.Default()),
	)

	ctx := context.Background()
	data, err := os.ReadFile("photo.jpg")
	if err != nil {
		log.Fatal(err)
	}

	media, err := client.UploadMedia(ctx, data, &ilink.UploadOptions{FileName: "photo.jpg"})
	if err != nil {
		log.Fatal(err)
	}

	// Embed the uploaded MediaItem in an outbound message.
	err = client.Send(ctx, &ilink.OutboundMessage{
		To:           "o9cq800kum_xxx@im.wechat",
		ContextToken: "<context_token>",
		Items:        []ilink.Item{{Type: ilink.ItemTypeImage, Image: media}},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// ExampleNewClient_options shows available client options.
func ExampleNewClient_options() {
	token := ilink.Token{BotToken: "your-bot-token"}

	client := ilink.NewClient(token,
		ilink.WithLogger(slog.Default()),
	)

	_ = client
}
