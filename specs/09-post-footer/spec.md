# Spec 09 · 文章页脚增强（分享 + 打赏 + 版权声明）

- **分支**：`feature/post-footer` · **Wave 1** · **隔离等级**：中高
- **主模块（独占）**：`internal/template/partial/content.html` 正文后页脚区

## Why

文章正文结束后缺少：社交分享、赞赏/打赏、版权声明（署名协议 + 原文链接）。这些是个人博客标配，均可配置驱动，落在 content.html 页脚区 + 站点配置。

## What Changes

- `internal/template/partial/content.html`：在 `<article class="md-text content">` 之后、评论区之前，追加页脚块：
  - **版权声明**：作者、发布/更新时间、许可协议（默认 CC BY-NC-SA 4.0）、原文链接（`.Page.CanonicalURL`）。
  - **分享**：分享到微博/X/QQ/复制链接（纯链接跳转 + 复制按钮，无第三方 SDK）。
  - **打赏**：可选，展示收款码图（微信/支付宝），点击展开。
- `internal/config/site.go` + `config/site.yml`：新增 `article` 段：`copyright{enabled,license,license_url}`、`reward{enabled,wechat,alipay,text}`、`share{enabled,platforms}`。
- `internal/template/model.go`：`Site` 结构体 +`Article ArticleConfig` 字段；`ArticleConfig`/`RewardConfig` 等定义放**新文件** `internal/template/model_article.go`（减少 model.go 改动）。
- （可选）`assets/css/`：若 main.css 无 `.post-copyright`/`.reward` 类才补一个小 css；优先对照 Stellar `_partial/main/post/` 的 copyright/reward 结构复用现成类。

## 技术决策（无需再问）

- **配置驱动**：全部开关与内容走 `site.yml`，无 DB。
- **对齐 Stellar**：先看 `../resource/hexo-theme-stellar/layout/_partial/` 有无 copyright/share/reward 组件与 main.css 类，能复用则复用，样式用变量。
- **分享**：用平台分享 URL（`https://service.weibo.com/share/share.php?url=...&title=...` 等），`window.open`；复制链接用 clipboard API（回退 prompt）。
- **打赏图**：图片 URL 由配置提供（可指向 `/uploads/...`，与附件功能天然协作）。
- **打赏码懒展开**：点击「打赏」按钮再显示二维码，默认收起。

## Impact

- **新增**：`internal/template/model_article.go`（+可选小 css）
- **修改**：`content.html`（+页脚区）、`site.go`+`site.yml`（+article 段）、`model.go`（Site +1 字段）
- **不影响**：路由、store、admin、其它页面

## ADDED Requirements

### Requirement: 文章页脚
The system SHALL append copyright, share, and optional reward sections after article content, driven by `site.yml`.

#### Scenario: 版权声明
- **WHEN** 打开任意文章且 `article.copyright.enabled=true`
- **THEN** 正文后显示作者/时间/协议/原文链接

#### Scenario: 分享
- **WHEN** 点击分享按钮
- **THEN** 打开对应平台分享页；点「复制链接」复制当前 URL

#### Scenario: 打赏收起
- **WHEN** `reward.enabled=true`
- **THEN** 默认显示「打赏」按钮，点击后展开收款码

#### Scenario: 全关闭
- **WHEN** 三项配置均 false
- **THEN** 文章页脚不渲染任何额外块，布局不变

## 验证

1. `go build ./... && go vet ./...` 通过
2. 开关分别开/关，页脚按配置出现/消失
3. 分享跳转正确、复制链接可用
4. 亮/暗模式样式正常；移动端不溢出

## Out of Scope

- 真实支付/在线打赏（仅静态收款码）
- 分享数统计、海报生成
