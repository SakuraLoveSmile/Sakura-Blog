#!/usr/bin/env bash
# 博客自动更新脚本：检查 GitHub Release 最新版本，下载并替换二进制，重启服务。
# 用法：
#   手动执行：sudo /opt/blog/update.sh
#   自动执行：配合 blog-update.timer 定时运行
#
# 依赖：curl, sha256sum, systemctl

set -euo pipefail

# ---------- 配置 ----------
REPO="SakuraLoveSmile/Sakura-Blog"
INSTALL_DIR="/opt/blog"
BINARY_NAME="blog"
SERVICE_NAME="blog"
# GitHub 文件下载代理（国内加速，留空则直连）
PROXY="https://ghfast.top/"
# GitHub API（通常可直连，不行再加代理）
API_BASE="https://api.github.com"
# 日志
LOG_FILE="/opt/blog/data/update.log"

# ---------- 函数 ----------
log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"; }

get_latest_tag() {
  curl -sfL "${API_BASE}/repos/${REPO}/releases/latest" \
    | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4
}

get_current_version() {
  if [[ -f "${INSTALL_DIR}/version" ]]; then
    cat "${INSTALL_DIR}/version"
  else
    echo "unknown"
  fi
}

download_file() {
  local url="$1" dest="$2"
  if [[ -n "$PROXY" ]]; then
    curl -fSL --retry 3 --connect-timeout 15 -o "$dest" "${PROXY}${url}"
  else
    curl -fSL --retry 3 --connect-timeout 15 -o "$dest" "$url"
  fi
}

# ---------- 主流程 ----------
mkdir -p "$(dirname "$LOG_FILE")"

log "=== 开始检查更新 ==="

LATEST_TAG=$(get_latest_tag)
if [[ -z "$LATEST_TAG" ]]; then
  log "ERROR: 无法获取最新版本号（网络问题或仓库无 Release）"
  exit 1
fi

CURRENT=$(get_current_version)
log "当前版本: ${CURRENT}, 最新版本: ${LATEST_TAG}"

if [[ "$LATEST_TAG" == "$CURRENT" ]]; then
  log "已是最新版本，无需更新"
  exit 0
fi

# 下载二进制 + sha256
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/blog-linux-amd64"
SHA_URL="${DOWNLOAD_URL}.sha256"
TMP_BIN=$(mktemp)
TMP_SHA=$(mktemp)
trap 'rm -f "$TMP_BIN" "$TMP_SHA"' EXIT

log "下载 ${LATEST_TAG} ..."
download_file "$DOWNLOAD_URL" "$TMP_BIN"
download_file "$SHA_URL" "$TMP_SHA"

# 校验 sha256
EXPECTED_SHA=$(awk '{print $1}' "$TMP_SHA")
ACTUAL_SHA=$(sha256sum "$TMP_BIN" | awk '{print $1}')
if [[ "$EXPECTED_SHA" != "$ACTUAL_SHA" ]]; then
  log "ERROR: SHA256 校验失败！expected=${EXPECTED_SHA} actual=${ACTUAL_SHA}"
  exit 1
fi
log "SHA256 校验通过"

# 停止服务 → 替换二进制 → 启动服务
log "停止服务 ${SERVICE_NAME} ..."
systemctl stop "$SERVICE_NAME" || true

cp "$TMP_BIN" "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
echo "$LATEST_TAG" > "${INSTALL_DIR}/version"

log "启动服务 ${SERVICE_NAME} ..."
systemctl start "$SERVICE_NAME"

# 等待健康检查
sleep 3
if systemctl is-active --quiet "$SERVICE_NAME"; then
  log "更新成功：${CURRENT} → ${LATEST_TAG}"
else
  log "ERROR: 服务启动失败，请检查 journalctl -u ${SERVICE_NAME}"
  exit 1
fi
