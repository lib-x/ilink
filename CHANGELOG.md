# Changelog

所有版本变更记录在此文件。格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [Semantic Versioning](https://semver.org/)。

---

## [0.1.0] - 2026-05-09

### 上游版本
`@tencent-weixin/openclaw-weixin@1.0.2`

### 新增
- 首次发布
- `StartLogin` / `Login.Wait`：二维码扫码登录流程，返回可持久化的 `Token`
- `NewClient`：通过 `Token` 构造客户端，支持 `WithHTTPClient`、`WithLogger`、`WithBaseURL` 选项
- `Client.ListenAndServe`：长轮询消息循环，实现 `Handler` 接口即可接收消息
- `Handler` / `HandlerFunc`：对齐 `net/http.Handler` 风格的消息处理接口
- `Client.Send` / `Client.Reply`：发送文本、图片、语音、文件、视频消息
- `Client.SendTyping`：发送"正在输入"状态指示器
- `Client.UploadMedia`：AES-128-ECB 加密 + CDN 预签名上传，返回 `MediaItem`
- `DecryptMedia` / `DownloadAndDecrypt`：CDN 媒体解密工具函数
- `TextMessage` 便捷构造函数
- `Message.Text()`、`Message.IsGroup()` 辅助方法
- `APIError` 结构体及 `ErrQRCodeExpired`、`ErrQRCodeCancelled`、`ErrLoginTimeout` 哨兵错误
