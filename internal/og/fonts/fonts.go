// Package fonts 内嵌 OG 分享图渲染字体（LXGW WenKai 常规体）。
// 独立 embed 仅供 OG 渲染器使用，避免 19MB TTF 暴露在 /assets 公网（Spec 61 阶段 1）。
package fonts

import "embed"

//go:embed LXGWWenKai-Regular.ttf
var FS embed.FS
