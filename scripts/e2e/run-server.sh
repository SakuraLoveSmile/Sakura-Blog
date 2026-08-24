#!/usr/bin/env bash
set -euo pipefail

server_tmp="$(mktemp -d "$PWD/.tmp/e2e-server.XXXXXX")"
go run ./scripts/e2e/prepare.go -db "$server_tmp/blog.db"

BLOG_DB="$server_tmp/blog.db" \
BLOG_LISTEN="${E2E_LISTEN:-:8096}" \
BLOG_AUTO_BACKUP=false \
go run ./cmd/blog -root "$PWD" -dev
