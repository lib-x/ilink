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

// UploadOptions configures an [Client.UploadMedia] call.
type UploadOptions struct {
	// FileName is embedded in the CDN reference for file-type attachments.
	FileName string
	// ThumbData is raw thumbnail bytes for image or video uploads.
	// When set, a separate CDN slot is allocated for the thumbnail.
	ThumbData []byte
}

// UploadMedia encrypts data with a fresh AES-128 key, uploads the ciphertext
// to Tencent CDN, and returns a [MediaItem] ready to embed in an [Item].
//
//	data, _ := os.ReadFile("photo.jpg")
//	media, err := client.UploadMedia(ctx, data, &ilink.UploadOptions{FileName: "photo.jpg"})
//	if err != nil { ... }
//	err = client.Reply(ctx, msg, ilink.Item{Type: ilink.ItemTypeImage, Image: media})
func (c *Client) UploadMedia(ctx context.Context, data []byte, opts *UploadOptions) (*MediaItem, error) {
	if opts == nil {
		opts = &UploadOptions{}
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

	plainMD5, encSize, plainSize := md5hex(data), int64(len(enc)), int64(len(data))

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

	// 4. Get pre-signed CDN URLs.
	urlReq := map[string]any{
		"file_size":      plainSize,
		"file_md5":       plainMD5,
		"encrypted_size": encSize,
	}
	if thumbEnc != nil {
		urlReq["thumb_file_size"] = int64(len(opts.ThumbData))
		urlReq["thumb_file_md5"] = thumbMD5
		urlReq["thumb_encrypted_size"] = int64(len(thumbEnc))
	}

	var urlResp struct {
		Ret               int    `json:"ret"`
		UploadURL         string `json:"upload_url"`
		EncryptQueryParam string `json:"encrypt_query_param"`
		ThumbUploadURL    string `json:"thumb_upload_url,omitempty"`
		ThumbEncryptQuery string `json:"thumb_encrypt_query_param,omitempty"`
	}
	if err := c.postJSON(ctx, "/ilink/bot/getuploadurl", urlReq, &urlResp); err != nil {
		return nil, fmt.Errorf("ilink: UploadMedia: getuploadurl: %w", err)
	}
	if err := checkRet("getuploadurl", urlResp.Ret); err != nil {
		return nil, err
	}

	// 5. PUT encrypted bytes to CDN.
	if err := putBytes(ctx, urlResp.UploadURL, enc); err != nil {
		return nil, fmt.Errorf("ilink: UploadMedia: upload: %w", err)
	}
	if thumbEnc != nil && urlResp.ThumbUploadURL != "" {
		if err := putBytes(ctx, urlResp.ThumbUploadURL, thumbEnc); err != nil {
			return nil, fmt.Errorf("ilink: UploadMedia: upload thumb: %w", err)
		}
	}

	return &MediaItem{
		FileName:          opts.FileName,
		FileSize:          plainSize,
		FileMD5:           plainMD5,
		EncryptedSize:     encSize,
		AESKey:            base64.StdEncoding.EncodeToString(aesKey),
		EncryptQueryParam: urlResp.EncryptQueryParam,
	}, nil
}

// DecryptMedia decrypts a CDN-downloaded encrypted blob using the AES key
// stored in the [MediaItem]. Use this to read images/files received from users.
//
//	data, _ := downloadCDN(msg.Items[0].Image)
//	plain, err := ilink.DecryptMedia(msg.Items[0].Image, data)
func DecryptMedia(item *MediaItem, ciphertext []byte) ([]byte, error) {
	aesKey, err := base64.StdEncoding.DecodeString(item.AESKey)
	if err != nil {
		return nil, fmt.Errorf("ilink: DecryptMedia: decode key: %w", err)
	}
	return decryptECB(aesKey, ciphertext)
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

// DownloadAndDecrypt is a convenience function that fetches ciphertext from a
// CDN URL and decrypts it using the key stored in item.
//
// The caller is responsible for constructing the full CDN URL from the
// EncryptQueryParam field (the scheme and host are
// https://novac2c.cdn.weixin.qq.com/c2c).
func DownloadAndDecrypt(ctx context.Context, cdnURL string, item *MediaItem) ([]byte, error) {
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
