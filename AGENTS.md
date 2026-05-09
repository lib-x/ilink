# AGENTS.md — AI Agent 维护指南

本文件是给 AI Agent 看的操作手册。当 `@tencent-weixin/openclaw-weixin` 发布新版本时，
按照本文件的步骤将变更同步到本 Go SDK（`github.com/lib-x/ilink`）。

---

## 0. 前置概念：上下游对应关系

```
上游 (TypeScript / npm)                    本库 (Go)
─────────────────────────────────────────────────────────────
src/api/types.ts      ──wire 类型──▶  wire.go   (内部)
                      ──公开类型──▶  message.go
src/api/api.ts        ──HTTP 调用──▶  transport.go / client.go
src/auth/             ──登录流程──▶  auth.go
src/cdn/              ──加解密────▶  media.go
src/monitor/          ──轮询循环──▶  client.go ListenAndServe
src/config/           ──配置字段──▶  client.go Option 函数
```

---

## 1. 检查上游是否有新版本

```bash
# 查当前最新版本号
npm view @tencent-weixin/openclaw-weixin version

# 查变更历史（changelog / git tag）
npm view @tencent-weixin/openclaw-weixin dist-tags
```

同时查看官方仓库的 Release / Commit 记录：
- https://github.com/Tencent/openclaw-weixin/releases
- https://www.npmjs.com/package/@tencent-weixin/openclaw-weixin?activeTab=versions

将最新版本号与 `go.mod` 顶部注释中记录的 `# upstream: x.y.z` 对比。
若版本相同则无需更新，终止流程。

---

## 2. 下载并解包上游源码

```bash
# 下载 tgz 并解压到临时目录
npm pack @tencent-weixin/openclaw-weixin
tar -xzf tencent-weixin-openclaw-weixin-*.tgz -C /tmp/upstream
```

关键文件清单（优先阅读这几个）：

| 文件 | 关注点 |
|------|--------|
| `package/src/api/types.ts` | 所有消息 / 媒体类型定义 |
| `package/src/api/api.ts` | API endpoint、请求体、响应体 |
| `package/src/auth/qrcode.ts` | 登录流程、状态字符串 |
| `package/src/cdn/encrypt.ts` | 加密算法和参数 |
| `package/src/monitor/poller.ts` | 轮询逻辑、退避策略 |
| `CHANGELOG.md` 或 `package/CHANGELOG.md` | 摘要式变更说明 |

---

## 3. 差异分析（逐文件对比）

对每个关键文件执行语义级 diff，记录以下四类变更：

### 3.1 新增字段（最常见）

查找 TypeScript interface 中新增的属性：

```typescript
// 示例：types.ts 新增了 recalled 字段
interface WeixinMessage {
  // ...
  recalled?: boolean   // ← 新增
}
```

对应操作：
- 在 `wire.go` 的 `wireMessage` struct 加同名 JSON 字段
- 若字段对调用方有用，同步在 `message.go` 的 `Message` struct 中暴露
- 在 `wire.go` 的 `convertMessage` 函数中赋值

### 3.2 新增 API endpoint

在 `api.ts` 中查找新的函数或 fetch 调用：

```typescript
// 示例：新增了撤回消息接口
async recallMessage(msgId: string) {
  return this.post('/ilink/bot/recallmessage', { msg_id: msgId })
}
```

对应操作：
- 在 `client.go` 新增对应的公开方法
- 若有复杂请求/响应结构，在 `wire.go` 新增内部 wire 结构体
- 编写单元测试（见第 5 节）

### 3.3 字段改名 / 类型变更（破坏性）

```typescript
// 旧：get_updates_buf string
// 新：cursor string
```

对应操作：
- 更新 `wire.go` 中的 JSON tag
- **不改变 Go 公开 API**（Go struct 字段名保持语义名称，JSON tag 跟随上游）
- 在 `CHANGELOG.md` 注明兼容性影响

### 3.4 认证头 / 加密算法变更（高风险）

重点检查：
- `src/auth/` 中请求头的构造方式（`X-WECHAT-UIN`、`AuthorizationType` 等）
- `src/cdn/encrypt.ts` 中的加密模式（AES key 长度、padding 方式）

若有变更，对应修改 `transport.go` 或 `media.go`，并更新 `ilink_test.go` 中的加密测试。

---

## 4. 执行更新

按以下顺序修改，每步完成后单独 `go build` 确认无编译错误。

```
Step 1  wire.go      — 更新内部 wire 结构体的 JSON 字段
Step 2  message.go   — 若有需要暴露给调用方的新字段/类型，在此添加
Step 3  wire.go      — 更新 convertMessage / marshalItems 转换逻辑
Step 4  client.go    — 新增/修改公开方法
Step 5  transport.go — 若认证头有变更
Step 6  media.go     — 若加密算法有变更
Step 7  auth.go      — 若登录状态字符串或流程有变更
```

修改完成后，更新 `go.mod` 顶部的版本注释：

```
// upstream: @tencent-weixin/openclaw-weixin@<新版本号>
```

---

## 5. 测试要求

每次更新必须保证以下全部通过：

```bash
go vet ./...
go build ./...
go test -v -count=1 -race ./...
```

### 新增测试的规范

- 新增 API endpoint → 在 `ilink_test.go` 新增对应的 `Test<功能名>` 函数
- 新增字段 → 在 `TestConvertMessage*` 系列中覆盖新字段的转换
- 加密算法变更 → 扩展 `TestEncryptDecryptECB` 的 case 列表
- 测试命名：`Test<被测函数><场景>`，例如 `TestConvertMessageRecalled`

禁止删除已有测试（除非对应功能在上游被移除）。

---

## 6. 版本号管理

本库遵循 [Semantic Versioning](https://semver.org/)：

| 上游变更类型 | 本库版本 bump |
|-------------|--------------|
| 新增字段 / 新增 endpoint（向后兼容） | `MINOR` (v0.x.0 → v0.x+1.0) |
| 字段改名 / 删除公开 API（破坏性） | `MAJOR` (v1.x.x → v2.0.0) |
| Bug fix / 文档更新 | `PATCH` (v0.x.y → v0.x.y+1) |

Major bump 需同步更新 `go.mod` 的 module path：
```
module github.com/lib-x/ilink/v2
```

---

## 7. 更新 CHANGELOG.md

在 `CHANGELOG.md` 顶部插入新条目（格式见下）：

```markdown
## [0.x.0] - YYYY-MM-DD

### 上游版本
@tencent-weixin/openclaw-weixin@x.y.z

### 新增
- `Client.RecallMessage` 方法，对应 `/ilink/bot/recallmessage`
- `Message.Recalled` 字段

### 变更
- `wireMessage.get_updates_buf` JSON tag 改为 `cursor`（内部，不影响公开 API）

### 修复
- （若有）
```

---

## 8. 完整执行检查清单

在提交 PR 或合并前，逐项打勾：

- [ ] `npm view` 确认上游版本号
- [ ] 下载并解包上游 tgz
- [ ] 阅读 `CHANGELOG.md` / commit log，归类所有变更
- [ ] `wire.go` JSON tag 与上游 types.ts 对齐
- [ ] `message.go` 公开字段已更新（仅暴露调用方需要的）
- [ ] `convertMessage` / `marshalItems` 转换逻辑正确
- [ ] 新增 endpoint 已实现并有对应公开方法
- [ ] 认证头构造与上游 `api.ts` 一致
- [ ] 加密参数与上游 `encrypt.ts` 一致
- [ ] `go vet ./...` 通过
- [ ] `go build ./...` 通过
- [ ] `go test -v -count=1 -race ./...` 全绿（含 Example* 函数）
- [ ] `go.mod` 顶部注释版本号已更新
- [ ] `CHANGELOG.md` 已新增条目
- [ ] 版本 bump 类型正确（MAJOR / MINOR / PATCH）

---

## 9. 高风险变更的额外步骤

以下变更需要人工审核，不可由 Agent 自动合并：

1. **认证机制变更**（新增 / 移除请求头、Token 格式变化）
   - 需手工在真实 iLink 环境扫码验证

2. **加密算法变更**（从 ECB 切换到其他模式、key 长度变化）
   - 需用上游 TypeScript Demo 与本库 Go 实现做交叉验证

3. **长轮询协议变更**（timeout、cursor 机制改变）
   - 需压测确认不会出现消息重复或丢失

4. **API 移除**（上游删除了某个 endpoint）
   - 需评估是否作 deprecated 过渡，不可直接删除公开方法

遇到以上情形，在 PR description 中标注 `⚠️ 需要人工验证`，并停止自动合并。

---

## 附录：文件职责速查

```
github.com/lib-x/ilink/
├── ilink.go          包文档、APIError、哨兵错误
├── message.go        公开类型：Item / Message / Handler / HandlerFunc / OutboundMessage
├── auth.go           Token / Login / StartLogin / Login.Wait
├── client.go         Client / Option / NewClient / ListenAndServe / Send / Reply / SendTyping
├── transport.go      HTTP 层：认证 header、JSON 编解码、CDN PUT
├── media.go          UploadMedia / DecryptMedia / AES-128-ECB 实现
├── wire.go           内部 wire 类型（不导出）+ 双向转换函数
├── util.go           零散内部工具函数
├── ilink_test.go     单元测试
├── example_test.go   godoc Example* 函数（go test 可执行、pkg.go.dev 可渲染）
├── go.mod            模块声明（顶部注释记录上游版本）
├── CHANGELOG.md      版本历史
└── AGENTS.md         本文件
```
