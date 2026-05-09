## [0.2.0] - 2026-05-09

### 上游版本
@tencent-weixin/openclaw-weixin@2.1.3

### 新增

- `CDNMedia` 类型（替代旧的扁平化 `MediaItem` 字段），新增 `EncryptType` 和 `FullURL` 字段，对齐上游 `CDNMedia` interface
- `ImageItem` 完全重构：持有 `Media *CDNMedia`、`ThumbMedia *CDNMedia` 以及 `AESKey`（raw hex）、`URL`、`ThumbWidth`/`ThumbHeight`/`ThumbSize`/`MidSize`/`HDSize` 等尺寸信息
- `VoiceItem` 完全重构：持有 `Media *CDNMedia`、`EncodeType VoiceEncodeType`、`SampleRate`、`PlaytimeMs`、`RecognText`（ASR 转写），移除旧的内嵌 `MediaItem`
- `FileItem` 完全重构：持有 `Media *CDNMedia`、`FileName`、`MD5`、`Size int64`
- `VideoItem` 完全重构：持有 `Media *CDNMedia`、`ThumbMedia *CDNMedia` 以及 `VideoSize`、`PlayLengthMs`、`VideoMD5`
- `VoiceEncodeType` 常量（`VoiceEncodePCM` 至 `VoiceEncodeOGGSpeex`）
- `RefMessage` 类型，`Item.RefMsg *RefMessage` 字段，支持引用消息
- `Item` 新增字段：`MsgID`、`CreateTimeMs`、`UpdateTimeMs`、`IsCompleted`（均来自上游 `MessageItem`）
- `Message` 新增字段：`Seq`、`MessageID`、`SessionID`、`CreateTimeMs`、`UpdateTimeMs`（均来自上游 `WeixinMessage`）
- `ErrSessionTimeout`：当服务器返回 `errcode=-14` 时，`ListenAndServe` 返回此错误，提示调用方需要重新登录
- `Client.RecallMessage(ctx, msgID int64) error`：对应 `/ilink/bot/recallmessage`
- `UploadOptions.MediaType`：`UploadMediaTypeImage/Video/File/Voice` 常量，对应上游 `UploadMediaType`
- `UploadOptions.ToUserID`：传给 `getuploadurl` 的目标用户 ID
- `UploadOptions.NoThumb`：抑制缩略图上传 slot（对应上游 `no_need_thumb`）

### 变更

- `getuploadurl` 请求字段名对齐上游：`file_size`→`rawsize`，`file_md5`→`rawfilemd5`，`encrypted_size`→`filesize`，新增 `media_type`、`aeskey`、`filekey`
- `getuploadurl` 响应字段名对齐上游：`upload_url`→`upload_param`，`thumb_upload_url`→`thumb_upload_param`，新增 `upload_full_url`
- `Client.UploadMedia` 返回类型从 `*MediaItem` 改为 `*CDNMedia`（`MediaItem` 现为 `CDNMedia` 的别名，向后兼容）
- `sendTyping` 的 status 编码修正：`1=typing`，`2=cancel`（原实现 `0/1` 与上游不符）
- `channelVersion` 更新为 `"2.0.0"`
- `ListenAndServe` 现在尊重服务器返回的 `longpolling_timeout_ms` 作为动态轮询间隔提示
- `wire.go` 中所有媒体 wire 类型完全重写，从扁平字段改为 `*wireCDNMedia` 嵌套结构
- `DecryptMedia`、`DownloadAndDecrypt` 参数类型从 `*MediaItem` 改为 `*CDNMedia`（因 `MediaItem = CDNMedia`，调用方无需修改）

### 修复

- `sendTyping` status 值错误（旧实现 `status=0` 无法被服务器识别为「停止输入」）

### 破坏性变更（内部 wire 层，不影响公开 API）

- `wireItem` 中 `ImageItem`/`VoiceItem`/`FileItem`/`VideoItem` 字段类型全部重写，旧的 `wireMediaItem`/`wireVoiceItem`（嵌入式）已移除

---

## [0.1.0] - 2026-05-09

初始发布。
