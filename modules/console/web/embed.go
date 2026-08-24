// Package console 嵌入 Vue Console SPA 的构建产物（dist/），部署时随二进制分发，
// 无需外部 console/dist 目录。embed 路径只能嵌包目录子树，故 embed.go 放在 console/ 内。
package console

import "embed"

// DistFS 嵌入的 console 构建产物文件系统（dist/ 子树），供 main.go 托管 /console/ 路由。
// 用 all: 前缀包含下划线/点开头的文件。
//
// 注意：编译期要求 dist/ 已存在且非空，故 go build 前必须先执行 `pnpm --filter console run build`。
//
//go:embed all:dist
var DistFS embed.FS
