#!/usr/bin/env bash
set -euo pipefail

mkdir -p "$PWD/.tmp"
server_tmp="$(mktemp -d "$PWD/.tmp/e2e-server.XXXXXX")"
# 仅共享当前构建的界面资源；数据库、附件、site.yml 和内容均位于测试根目录。
# 避免在 dev 运行恢复用例时读写用户真实配置和上传目录。
mkdir -p "$server_tmp/config" "$server_tmp/content/posts"
ln -s "$PWD/modules" "$server_tmp/modules"
ln -s "$PWD/internal" "$server_tmp/internal"
go run ./scripts/e2e/prepare.go -db "$server_tmp/data/blog.db"

BLOG_ROOT="$server_tmp" \
BLOG_DB="$server_tmp/data/blog.db" \
BLOG_LISTEN="${E2E_LISTEN:-:8096}" \
BLOG_AUTO_BACKUP=false \
BLOG_MAINTENANCE_LOCK="$server_tmp/data/maintenance.lock" \
go run ./cmd/blog -root "$server_tmp" -config "$server_tmp/config/app.yml" -dev
