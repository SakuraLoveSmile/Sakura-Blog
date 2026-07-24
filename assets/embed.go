// Package assets 嵌入静态资源（CSS/JS/字体），部署时随二进制分发，无需外部 assets 目录。
// embed 路径只能嵌包目录子树，故 embed.go 放在 assets/ 内，嵌入 css/js/fonts 三个子目录。
package assets

import "embed"

// FS 嵌入的静态资源文件系统，供 main.go 用 http.FS 挂到 /assets 路由。
// 用 all: 前缀包含下划线/点开头的文件（如字体子集）。
//
//go:embed all:css all:js all:fonts all:img manifest.webmanifest offline.html
var FS embed.FS
