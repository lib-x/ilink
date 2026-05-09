package ilink

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
)

// UploadMediaType constants mirror UploadMediaType from upstream types.ts.
const (
	UploadMediaTypeImage = 1
	UploadMediaTypeVideo = 2
	UploadMediaTypeFile  = 3
	UploadMediaTypeVoice = 4
)

// UploadOptions configures a [Client.UploadMedia] call.
type UploadOptions struct {
	// FileName is embedded in the CDN reference for file-type attachments.
	FileName string

	// MediaType selects the upload slot: use the UploadMediaType* constants.
	// Defaults to UploadMediaTypeImage when zero.
	MediaType int

	// ToUserID is the target recipient's user ID. Required for routing when
	// the server needs it to resolve the CDN bucket.
	ToUserID string

	// ThumbData is raw thumbnail bytes for IMAGE or VIDEO uploads.
	// When set, a separate CDN slot is allocated for the thumbnail.
	ThumbData []byte

	// NoThumb suppresses thumbnail allocation even when ThumbData is empty.
	NoThumb bool
}

// UploadMedia encrypts data with a fresh AES-128 key, uploads the ciphertext
// to Tencent CDN, and returns a [CDNMedia] ready to embed in an [Item].
//
//	data, _ := os.ReadFile("photo.jpg")
//	media, err := client.UploadMedia(ctx, data, &ilink.UploadOptions{
//	    FileName:  "photo.jpg",
//	    MediaType: ilink.UploadMediaTypeImage,
//	    ToUserID:  msg.From,
//	})
//	if err != nil { ... }
//	err = client.Reply(ctx, msg, ilink.Item{
//	    Type:  ilink.ItemTypeImage,
//	    Image: &ilink.ImageItem{Media: media},
//	})
func (c *Client) UploadMedia(ctx context.Context, data []byte, opts *UploadOptions) (*CDNMedia, error) {
	if opts == nil {
		opts = &UploadOptions{}
	}
	mediaType := opts.MediaType
	if mediaType == 0 {
		mediaType = UploadMediaTypeImage
	}

	// 1. Generate a random 128-bit AES key.
	aesKey := make([]byte, 16)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, fmt.Errorf("ilink: UploadMedia: generate key: %w", err)
	}

	// 2. Encrypt with AES-128-ECB.
	enc, err := encryptECB(aesKey, data)
	if err != nil {
		return nil, fmt.Errorf("ilink: UploadMedia: encrypt: %w", err)
	}

	plainMD5 := md5hex(data)
	plainSize := int64(len(data))
	encSize := int64(len(enc))

	// 3. Optionally encrypt thumbnail.
	var thumbEnc []byte
	var thumbMD5 string
	if len(opts.ThumbData) > 0 {
		thumbEnc, err = encryptECB(aesKey, opts.ThumbData)
		if err != nil {
			return nil, fmt.Errorf("ilink: UploadMedia: encrypt thumb: %w", err)
		}
		thumbMD5 = md5hex(opts.ThumbData)
	}

	// 4. Get pre-signed CDN upload parameters (new field names from types.ts v2).
	urlReq := map[string]any{
		"media_type":   mediaType,
		"to_user_id":   opts.ToUserID,
		"rawsize":      plainSize,
		"rawfilemd5":   plainMD5,
		"filesize":     encSize,
		"aeskey":       base64.StdEncoding.EncodeToString(aesKey),
	}
	if opts.FileName != "" {
		urlReq["filekey"] = opts.FileName
	}
	if thumbEnc != nil {
		urlReq["thumb_rawsize"] = int64(len(opts.ThumbData))
		urlReq["thumb_rawfilemd5"] = thumbMD5
		urlReq["thumb_filesize"] = int64(len(thumbEnc))
	} else if opts.NoThumb {
		urlReq["no_need_thumb"] = true
	}

	var urlResp struct {
		Ret            int    `json:"ret"`
		UploadParam    string `json:"upload_param"`
		ThumbUploadParam string `json:"thumb_upload_param,omitempty"`
		UploadFullURL  string `json:"upload_full_url,omitempty"`
	}
	if err := c.postJSON(ctx, "/ilink/bot/getuploadurl", urlReq, &urlResp); err != nil {
		return nil, fmt.Errorf("ilink: UploadMedia: getuploadurl: %w", err)
	}
	if err := checkRet("getuploadurl", urlResp.Ret); err != nil {
		return nil, err
	}

	// 5. PUT encrypted bytes to CDN.
	uploadURL := urlResp.UploadFullURL
	if uploadURL == "" {
		uploadURL = urlResp.UploadParam
	}
	if err := putBytes(ctx, uploadURL, enc); err != nil {
		return nil, fmt.Errorf("ilink: UploadMedia: upload: %w", err)
	}
	if thumbEnc != nil {
		thumbURL := urlResp.ThumbUploadParam
		if thumbURL == "" {
			return nil, fmt.Errorf("ilink: UploadMedia: server did not return thumb_upload_param")
		}
		if err := putBytes(ctx, thumbURL, thumbEnc); err != nil {
			return nil, fmt.Errorf("ilink: UploadMedia: upload thumb: %w", err)
		}
	}

	return &CDNMedia{
		EncryptQueryParam: urlResp.UploadParam,
		AESKey:            base64.StdEncoding.EncodeToString(aesKey),
		FullURL:           urlResp.UploadFullURL,
	}, nil
}

// DecryptMedia decrypts a CDN-downloaded encrypted blob using the AES key
// stored in the [CDNMedia]. Use this to read images/files received from users.
//
//	data, _ := ilink.DownloadAndDecrypt(ctx, cdnURL, msg.Items[0].Image.Media)
//	// or, if you downloaded the bytes yourself:
//	plain, err := ilink.DecryptMedia(msg.Items[0].Image.Media, ciphertext)
func DecryptMedia(item *CDNMedia, ciphertext []byte) ([]byte, error) {
	aesKey, err := base64.StdEncoding.DecodeString(item.AESKey)
	if err != nil {
		return nil, fmt.Errorf("ilink: DecryptMedia: decode key: %w", err)
	}
	return decryptECB(aesKey, ciphertext)
}

// DownloadAndDecrypt is a convenience function that fetches ciphertext from a
// CDN URL and decrypts it using the key stored in item.
//
// When item.FullURL is populated the caller may use it directly; otherwise
// construct the URL from item.EncryptQueryParam and the CDN base:
//
//	https://novac2c.cdn.weixin.qq.com/c2c?<EncryptQueryParam>
func DownloadAndDecrypt(ctx context.Context, cdnURL string, item *CDNMedia) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cdnURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ilink: DownloadAndDecrypt: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ilink: DownloadAndDecrypt: fetch: %w", err)
	}
	defer resp.Body.Close()
	enc, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ilink: DownloadAndDecrypt: read: %w", err)
	}
	return DecryptMedia(item, enc)
}

// ---- AES-128-ECB helpers ----
//
// Go's crypto/aes does not expose ECB directly; we implement it by chaining
// single-block Encrypt calls with PKCS#7 padding.

func encryptECB(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	padded := pkcs7Pad(plaintext, bs)
	out := make([]byte, len(padded))
	for i := 0; i < len(padded); i += bs {
		block.Encrypt(out[i:i+bs], padded[i:i+bs])
	}
	return out, nil
}

func decryptECB(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	if len(ciphertext)%bs != 0 {
		return nil, fmt.Errorf("ilink: decryptECB: ciphertext length %d is not a multiple of block size %d", len(ciphertext), bs)
	}
	out := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += bs {
		block.Decrypt(out[i:i+bs], ciphertext[i:i+bs])
	}
	return pkcs7Unpad(out)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - (len(data) % blockSize)
	return append(data, bytes.Repeat([]byte{byte(pad)}, pad)...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("ilink: pkcs7Unpad: empty data")
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > aes.BlockSize {
		return nil, fmt.Errorf("ilink: pkcs7Unpad: invalid padding %d", pad)
	}
	return data[:len(data)-pad], nil
}

func md5hex(b []byte) string {
	h := md5.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
