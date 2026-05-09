// Package ilink provides a client for the Tencent WeChat iLink Bot API.
//
// iLink is Tencent's official WeChat personal account Bot API, exposed through
// the OpenClaw platform. It allows developers to receive and send WeChat
// messages over a standard HTTP/JSON long-poll interface.
//
// # Authentication
//
// Login is performed by scanning a QR code with the WeChat app. The resulting
// token must be persisted by the caller and supplied when constructing a Client.
//
//	login, err := ilink.StartLogin(ctx)
//	if err != nil { ... }
//	fmt.Println("Scan:", login.URL)
//
//	token, err := login.Wait(ctx)
//	if err != nil { ... }
//	// persist token.BotToken for reuse
//
// # Receiving messages
//
// Implement the [Handler] interface and call [Client.ListenAndServe]:
//
//	client := ilink.NewClient(token)
//	err := client.ListenAndServe(ctx, ilink.HandlerFunc(func(ctx context.Context, msg *ilink.Message) error {
//	    fmt.Println(msg.From, ":", msg.Text())
//	    return client.Reply(ctx, msg, ilink.TextMessage("你好！"))
//	}))
//
// # Sending messages
//
// Use [Client.Send] to send a message to any user:
//
//	err := client.Send(ctx, &ilink.OutboundMessage{
//	    To:           "o9cq800kum_xxx@im.wechat",
//	    ContextToken: msg.ContextToken,
//	    Items:        []ilink.Item{ilink.TextMessage("Hello")},
//	})
//
// # Media
//
// Images, files, voices and videos are uploaded to Tencent CDN with AES-128-ECB
// encryption before sending. Use [Client.UploadMedia] to prepare a [MediaItem].
package ilink

import "errors"

// Sentinel errors returned by the package.
var (
	// ErrQRCodeExpired is returned by Login.Wait when the QR code expires
	// before the user scans it.
	ErrQRCodeExpired = errors.New("ilink: QR code expired")

	// ErrQRCodeCancelled is returned by Login.Wait when the user cancels
	// the scan on the phone.
	ErrQRCodeCancelled = errors.New("ilink: QR code scan cancelled")

	// ErrLoginTimeout is returned by Login.Wait when the context deadline is
	// exceeded before the user confirms the scan.
	ErrLoginTimeout = errors.New("ilink: timed out waiting for QR code confirmation")

	// ErrSessionTimeout is returned by ListenAndServe when the server reports
	// errcode=-14, indicating the bot session has expired and re-login is
	// required.
	ErrSessionTimeout = errors.New("ilink: session timeout (re-login required)")
)

// APIError represents a non-zero ret code returned by the iLink API.
type APIError struct {
	// Ret is the error code returned by the server.
	Ret int
	// Op is the API endpoint that returned the error.
	Op string
}

func (e *APIError) Error() string {
	return "ilink: " + e.Op + " returned ret=" + itoa(e.Ret)
}

func checkRet(op string, ret int) error {
	if ret == 0 {
		return nil
	}
	return &APIError{Op: op, Ret: ret}
}
