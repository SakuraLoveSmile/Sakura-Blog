# AI 助手：对话模型与向量模型拆分独立接口

## 目标与范围

当前 `blog_extensions.ai` 把对话（Chat）与向量（Embedding）共用同一组 `base_url` / `api_key`，只靠 `model` / `embedding_model` 区分模型名。用户要求把两者拆开，各自使用独立的 API 接口（地址、密钥、模型名）。

- 目标：AI 助手设置页分别配置「对话模型」和「向量模型」两组独立接口；RAG 建库/检索只用向量接口，摘要/属性生成/问答回答只用对话接口。
- 范围：canonical `blog_extensions.ai` 结构、Go typed bridge 与消费者、Console AI 设置表单、相关测试与验收基线。
- 不涉及：更换模型供应商、增加新 AI 能力、改动公开端点协议、改动 CSS/主题、发布/部署。

## 已确认决定

- 2026-11-02 用户要求拆分向量模型与对话模型，两者独立 API 接口。未提出其他产品规则。
- 新结构采用嵌套对象 `ai.chat` / `ai.embedding`，各含 `base_url`、`api_key`、`model`。
- 向后兼容：旧版平铺字段 `ai.base_url`、`ai.api_key`、`ai.model`、`ai.embedding_model` 继续作为**逐字段回退**。新嵌套字段非空时优先；为空时取旧字段。这样已上线配置不需要一次性手工迁移，也不会因为只填一半而丢凭据。
- 不自动改写旧字段、不做隐式迁移；旧字段保留在原文中，用户在新字段全部填好后可自行在完整配置里清理。
- 公开端点语义不变：`AI 摘要` 读者自助仍只使用对话接口；`知识库问答` 的 BYOK 仍只替换最终回答的对话接口，向量接口始终由站长配置。
- 切换向量接口或向量模型会改变 embedding fingerprint，需重建知识库（沿用现有 `needsRebuild` 行为，不自动触发重建）。

## 字段契约

`blog_extensions.ai`：

```json
{
  "chat": {
    "base_url": "https://chat.example/v1",
    "api_key": "sk-chat",
    "model": "chat-model"
  },
  "embedding": {
    "base_url": "https://embedding.example/v1",
    "api_key": "sk-embedding",
    "model": "embedding-model"
  },
  "summary": { "enabled": false, "allow_user_api": false },
  "ask": { "enabled": false, "allow_user_llm": false }
}
```

- `chat`/`embedding` 缺省、`null` 或字段为空时回退到旧字段；旧字段也不存在时视为未配置。
- 逐字段回退矩阵：
  - `chat.base_url` ← `ai.base_url`
  - `chat.api_key` ← `ai.api_key`
  - `chat.model` ← `ai.model`
  - `embedding.base_url` ← `ai.base_url`
  - `embedding.api_key` ← `ai.api_key`
  - `embedding.model` ← `ai.embedding_model`
- 脱敏路径新增 `ai.chat.api_key`、`ai.embedding.api_key`，保留旧 `ai.api_key`；保存时 `****` 一律由服务端恢复对应路径的已存值。
- Go typed bridge（`config.AIConfig`）暴露 `Chat` / `Embedding` 两个 `AIEndpoint{BaseURL, APIKey, Model}`；开关字段与语义不变。

## 写入边界（并行）

| 范围 | 唯一写入者 | 文件 |
| --- | --- | --- |
| 后端 Go | backend-ai | `internal/config/reload_ai.go`、`internal/config/stellar_blog_runtime.go`、`internal/config/stellar_v2_publish.go`、对应 `_test.go`、`internal/rag/rag.go`、`internal/rag/*_test.go`、`modules/console/api/ai.go`、`modules/console/api/rag.go`、对应 `_test.go` |
| Console 前端 | console-ui | `modules/console/web/src/modules/settings/settings-canonical-fields.ts`、`modules/console/web/src/modules/assistant/AiSettings.vue`、`modules/console/tests/settings-canonical-fields.test.ts`、`tests/e2e/console-settings-canonical.spec.ts` |
| 任务记录 | lead | `docs/plans/ai-split-endpoints.md` |
| 验收基线 | verifier | `docs/baselines/ai-split-endpoints/` |

- 后端不修改 Console 响应 struct，因此不需要重新生成 `packages/api-client/src/generated.d.ts`；若确需改 API 类型，先由 lead 确认。
- 前端不修改 Go 文件；字段路径严格按上面的契约。
- 浏览器验收统一用 8096 端口与 e2e 隔离数据目录，不连接主工作区数据库。

## 运行资源

- 基线：根工作区 `dev` / `d1e34a97`（根工作区已有未提交文档与验收素材，全部保留）。
- worktree：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261102-feat-blg-20261102-07-ai-split-endpoints`
- 分支：`codex/feat/20261102-blg-20261102-07-ai-split-endpoints`；合入目标：根工作区 `dev`（提交/合入需用户授权）。
- 验收数据：`scripts/e2e/prepare.go` 的隔离空站；Playwright 配置 `webServer` 使用 8096。

## 验收场景

1. **旧配置不回归**：只保留旧平铺字段时，聊天调用走 `ai.base_url`+`ai.api_key`+`ai.model`，RAG 建库/检索走 `ai.base_url`+`ai.api_key`+`ai.embedding_model`；`make test` 中相关 Go 测试通过。
2. **新配置独立生效**：同时存在旧字段和新嵌套字段时，chat 请求只发往 `chat.base_url` 并携带 `chat.model`、`chat.api_key`；embedding 请求只发往 `embedding.base_url`，`embeddingFingerprint` 使用 embedding 地址与模型。
3. **半填不丢能力**：只填 `chat` 三项、`embedding` 留空时，问答仍可用旧 embedding 配置；只填 `embedding` 三项时，摘要/属性生成仍用旧 chat 配置。
4. **凭据不泄漏**：GET 脱敏同时覆盖旧 `ai.api_key`、`ai.chat.api_key`、`ai.embedding.api_key`；未修改的 `****` 保存后恢复原值；显式清空写入空值。
5. **Console 表单**：AI 设置页显示「对话模型」「向量模型」两组各三项（地址、密钥、模型）；仅改动一组时另一组与旧字段原文不变；`****` 占位保持；保存后刷新值保持。
6. **构建与浏览器**：`make test`、`make build` 通过；真实服务下 8096 登录 Console，AI 设置页桌面/390px 截图无溢出；保存后 API 返回的 canonical 文档包含嵌套字段且旧字段不变。
7. **RAG 实际链路**：隔离服务内用本地假 OpenAI 端点（chat/embedding 不同端口或不同路径）验证建库只用 embedding 接口、问答只用 chat 接口；至少由 Go 测试覆盖两条链路。

## 代码与验证记录

状态：待体验（实现与本地验证完成，待用户确认）

- 后端：已实现（`AIEndpoint{Chat,Embedding}`、canonical 新旧结构解析与逐字段回退、嵌套脱敏、RAG/Console 消费改造）。定向 `go test -count=1 ./internal/config ./internal/rag ./modules/console/api ./internal/stellarmigration` exit 0；`go vet` exit 0。
- 前端：已实现（AI 设置四组：对话模型 / 向量模型 / AI 摘要 / AI 问答；单元测试 425/425 通过）。Lead 复核后修正 API Key 帮助文案为「留空回退旧 key」，并补 prettier。
- `make test`：在最终源码上通过（exit 0；Console 行为测试 238 pass / 2 skipped / 0 fail；go vet、staticcheck、全部 go test 通过，2026-11-02 20:43）。
- `make build`：在最终源码上通过（exit 0；Console 入口 `assets/index-nox47CgV.js`，`dist/index.html` sha256 `c08152cb…5f7b`；与 verifier 两次独立构建一致，2026-11-02 20:44）。
- e2e：`tests/e2e/console-settings-canonical.spec.ts` 11/11 通过（含 AI 拆分用例；20:25 官方 8096 首跑、20:35–36 最终源码隔离端口 8099 复跑）。
- 浏览器验收：独立 Playwright 2/2 通过（8097 隔离服务）：1280/390 无横向滚动；两组各三项；真实保存后 `chat`/`embedding` 嵌套字段按输入持久化、三个 `api_key` 均为 `****`、旧平铺字段与无关字段保留、大整数原文保留；刷新后保持；浏览器加载入口与 `dist/index.html` 引用字节一致。
- 验收基线：`docs/baselines/ai-split-endpoints/`（README、verification.json、db-evidence.json、截图与参照图）。
- 未提交、未合入 `dev`、未推送、未发布；提交与合入需用户授权。

## 遗留与边界

- 旧平铺字段不会自动删除；其长期存在不影响新配置，但完整配置里会有两组字段。
- 不在本次内：为旧配置提供一键迁移按钮、修改公开 API 文档的请求结构、额外展示对话模型到知识库状态卡。
