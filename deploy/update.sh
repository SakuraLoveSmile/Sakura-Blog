#!/usr/bin/env bash
# 博客自动更新脚本：检查 GitHub Release 最新版本，下载并替换二进制，重启服务。
# 覆盖安装前备份上一版本二进制；健康检查（本地端口 HTTP 探测）失败时自动回滚并重启。
# 用法：
#   手动执行：sudo /opt/blog/update.sh
#   自动执行：配合 blog-update.timer 定时运行
#
# 依赖：curl, sha256sum, systemctl, python3

set -euo pipefail

# ---------- 配置 ----------
REPO="SakuraLoveSmile/Sakura-Blog"
INSTALL_DIR="/opt/blog"
BINARY_NAME="blog"
SERVICE_NAME="blog"
# GitHub API（私有仓库需 Token 认证）
API_BASE="https://api.github.com"
# 私有仓库 Release 下载需要 PAT（权限 repo），写入 /opt/blog/.token（chmod 600）
TOKEN_FILE="${INSTALL_DIR}/.token"
# 日志文件
LOG_FILE="/opt/blog/data/update.log"
# 回滚备份：覆盖安装前备份上一版本二进制，健康检查失败时还原
BACKUP_BIN="${INSTALL_DIR}/${BINARY_NAME}.bak"
# 健康检查：对本地服务端口做 HTTP 探测（与 blog.service 的 BLOG_LISTEN 保持一致）
HEALTH_PORT="${BLOG_HEALTH_PORT:-8080}"
HEALTH_RETRIES="${BLOG_HEALTH_RETRIES:-5}"
# HTTP 代理（国内服务器走代理加速 GitHub，留空则直连）
# systemd timer 触发时无继承的环境变量，必须在此显式设置
PROXY="${BLOG_PROXY:-http://127.0.0.1:7890}"
if [[ -n "$PROXY" ]]; then
  export http_proxy="$PROXY"
  export https_proxy="$PROXY"
fi

# ---------- 颜色与输出函数 ----------
# 终端颜色（非终端或 NO_COLOR 环境变量时自动禁用）
if [[ -t 1 ]] && [[ -z "${NO_COLOR:-}" ]]; then
  C_RESET=$'\033[0m'
  C_BOLD=$'\033[1m'
  C_GREEN=$'\033[32m'
  C_RED=$'\033[31m'
  C_YELLOW=$'\033[33m'
  C_CYAN=$'\033[36m'
  C_GRAY=$'\033[90m'
else
  C_RESET="" C_BOLD="" C_GREEN="" C_RED="" C_YELLOW="" C_CYAN="" C_GRAY=""
fi

# 日志只写文件（终端输出由专门函数处理，避免混杂）
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$LOG_FILE"
}

# 步骤标题: [n/total] 描述
step() {
  local n="$1" total="$2" desc="$3"
  printf '%s[%s/%s]%s %s\n' "$C_BOLD" "$n" "$total" "$C_RESET" "$desc"
}

# 成功标记: ✓ msg
ok() {
  printf '  %s✓%s %s\n' "$C_GREEN" "$C_RESET" "${1:-}"
}

# 失败标记: ✗ msg（输出到 stderr）
fail() {
  printf '  %s✗%s %s\n' "$C_RED" "$C_RESET" "${1:-}" >&2
}

# 子项信息: desc ......... result（带对齐点）
sub() {
  local desc="$1" result="${2:-}"
  local width=40
  local dots_len=$((width - ${#desc}))
  [[ $dots_len -lt 3 ]] && dots_len=3
  local dots
  dots=$(printf '%*s' "$dots_len" '')
  dots="${dots// /.}"
  if [[ -n "$result" ]]; then
    printf '      %s%s%s %s%s%s %s\n' "$desc" "$C_GRAY" "$dots" "$C_RESET" "$result" "$C_RESET" ""
  else
    printf '      %s\n' "$desc"
  fi
}

# 分隔线
hr() {
  printf '%s%s%s\n' "$C_GRAY" "$(printf '%.0s─' {1..52})" "$C_RESET"
}

# 错误退出: 输出失败信息并 exit 1
die() {
  fail "$1"
  log "ERROR: $1"
  exit 1
}

# ---------- 核心函数 ----------
# 读取 GitHub Token（文件优先，环境变量兜底）
get_token() {
  if [[ -s "$TOKEN_FILE" ]]; then
    cat "$TOKEN_FILE"
  elif [[ -n "${BLOG_GH_TOKEN:-}" ]]; then
    echo "$BLOG_GH_TOKEN"
  else
    echo ""
  fi
}

# 查询 GitHub Release 最新版本号（含 3 次重试，不污染 stdout）
get_latest_tag() {
  local BLOG_QUIET=1
  local retries=3
  local result=""

  while [[ $retries -gt 0 ]]; do
    local tmp_response
    tmp_response=$(mktemp)
    local http_code=""

    if [[ -n "$AUTH_HEADER" ]]; then
      http_code=$(curl -s -o "$tmp_response" -w "%{http_code}" \
        --max-time 30 --connect-timeout 10 -H "$AUTH_HEADER" \
        "${API_BASE}/repos/${REPO}/releases/latest" 2>/dev/null || echo "000")
    else
      http_code=$(curl -s -o "$tmp_response" -w "%{http_code}" \
        --max-time 30 --connect-timeout 10 \
        "${API_BASE}/repos/${REPO}/releases/latest" 2>/dev/null || echo "000")
    fi

    if [[ "$http_code" == "200" ]]; then
      result=$(grep -o '"tag_name": *"[^"]*"' "$tmp_response" 2>/dev/null | head -1 | cut -d'"' -f4 || echo "")
    else
      log "API 返回 HTTP ${http_code}（剩余重试 ${retries}）"
      [[ "$http_code" == "404" ]] && log "仓库无 Release，请先在 GitHub 创建"
      [[ "$http_code" == "403" ]] && log "API 限流或 Token 权限不足"
    fi

    rm -f "$tmp_response"

    if [[ -n "$result" ]]; then
      echo "$result"
      return 0
    fi

    retries=$((retries - 1))
    [[ $retries -gt 0 ]] && sleep 5
  done

  echo ""
  return 1
}

# 获取当前已安装版本
get_current_version() {
  if [[ -f "${INSTALL_DIR}/version" ]]; then
    cat "${INSTALL_DIR}/version"
  else
    echo "unknown"
  fi
}

# 从 Release 获取指定资产的 API URL
# 私有仓库网页 URL 通过代理下载会 404，必须用 API URL（/releases/assets/{id}）
get_asset_url() {
  local tag="$1" filename="$2"
  local tmp_response
  tmp_response=$(mktemp)
  local asset_url=""

  if [[ -n "$AUTH_HEADER" ]]; then
    curl -sfL --max-time 30 -H "$AUTH_HEADER" \
      "${API_BASE}/repos/${REPO}/releases/tags/${tag}" > "$tmp_response" 2>/dev/null
  else
    curl -sfL --max-time 30 \
      "${API_BASE}/repos/${REPO}/releases/tags/${tag}" > "$tmp_response" 2>/dev/null
  fi

  # 用 python3 解析 JSON 提取资产的 API URL
  asset_url=$(python3 -c "
import json
try:
    data = json.load(open('$tmp_response'))
    for a in data.get('assets', []):
        if a.get('name') == '$filename':
            print(a.get('url', ''))
            break
except: pass
" 2>/dev/null)

  rm -f "$tmp_response"
  echo "$asset_url"
}

# 健康检查：systemd 处于 active 状态，且本地端口 HTTP 探测返回 2xx/3xx
# 注意：脚本全局导出了 http_proxy，本地探测必须 --noproxy 绕过代理
check_health() {
  systemctl is-active --quiet "$SERVICE_NAME" || return 1
  local code=""
  code=$(curl -s -o /dev/null -w "%{http_code}" --noproxy '*' \
    --max-time 5 "http://127.0.0.1:${HEALTH_PORT}/" 2>/dev/null || echo "000")
  [[ "$code" =~ ^(200|301|302|303|307|308)$ ]]
}

# 回滚：还原备份的上一版本二进制与版本号，并重启服务
rollback() {
  echo "  ${C_YELLOW}↩ 健康检查失败，回滚到 ${CURRENT} ...${C_RESET}"
  log "健康检查失败，开始回滚: ${LATEST_TAG} -> ${CURRENT}"
  systemctl stop "$SERVICE_NAME" 2>/dev/null || true
  if cp "$BACKUP_BIN" "${INSTALL_DIR}/${BINARY_NAME}" && chmod +x "${INSTALL_DIR}/${BINARY_NAME}"; then
    echo "$CURRENT" > "${INSTALL_DIR}/version"
    sub "还原旧版本二进制" "${C_GREEN}✓${C_RESET}"
    systemctl start "$SERVICE_NAME" 2>/dev/null || true
    sleep 3
    if check_health; then
      sub "回滚后健康检查" "${C_GREEN}✓${C_RESET} 已恢复 ${CURRENT}"
      log "回滚成功: ${LATEST_TAG} -> ${CURRENT}"
    else
      sub "回滚后健康检查" "${C_RED}✗${C_RESET} 服务仍未恢复，需人工介入"
      log "ERROR: 回滚后服务仍未恢复"
    fi
  else
    sub "还原旧版本二进制" "${C_RED}✗${C_RESET} 无法还原备份"
    log "ERROR: 回滚失败，无法还原备份二进制"
  fi
}

# 下载文件（静默模式，不显示 curl 进度条）
download_file() {
  local url="$1" dest="$2"
  log "下载: ${url}"

  if [[ -n "$AUTH_HEADER" ]]; then
    curl -fsSL --retry 3 --connect-timeout 30 --max-time 120 \
      -H "$AUTH_HEADER" -H "Accept: application/octet-stream" \
      -o "$dest" "$url" || return 1
  else
    curl -fsSL --retry 3 --connect-timeout 30 --max-time 120 \
      -o "$dest" "$url" || return 1
  fi

  local file_size
  file_size=$(stat -c%s "$dest" 2>/dev/null || stat -f%z "$dest" 2>/dev/null)
  numfmt --to=iec-i --suffix=B "$file_size" 2>/dev/null || echo "${file_size}B"
}

# ---------- 主流程 ----------
mkdir -p "$(dirname "$LOG_FILE")"
log "=== 开始检查更新 ==="

# 读取 Token
TOKEN=$(get_token)
AUTH_HEADER=""
[[ -n "$TOKEN" ]] && AUTH_HEADER="Authorization: token ${TOKEN}"

# 头部信息
echo ""
hr
echo "  ${C_BOLD}${C_CYAN}Blog 自动更新${C_RESET}"
echo "  ${C_GRAY}仓库${C_RESET}    ${REPO}"
hr
echo ""

# [1/5] API 连通性检查
step 1 5 "API 连通性检查"
test_response=$(mktemp)
http_code=$(curl -s -o "$test_response" -w "%{http_code}" --max-time 30 \
  ${AUTH_HEADER:+-H "$AUTH_HEADER"} "${API_BASE}/repos/${REPO}/releases/latest" 2>/dev/null || echo "000")
rm -f "$test_response"

if [[ "$http_code" == "200" ]]; then
  ok "API 可达 (HTTP 200)"
  log "API 连通性检查通过"
elif [[ "$http_code" == "404" ]]; then
  die "仓库无 Release 或 Token 无权限 (HTTP 404)"
elif [[ "$http_code" == "403" ]]; then
  die "API 限流或 Token 权限不足 (HTTP 403)"
else
  die "API 不可达 (HTTP ${http_code})"
fi
echo ""

# [2/5] 获取当前版本
step 2 5 "获取当前版本"
CURRENT=$(get_current_version)
ok "当前版本: ${C_YELLOW}${CURRENT}${C_RESET}"
echo ""

# [3/5] 查询最新版本
step 3 5 "查询 GitHub 最新版本"
LATEST_TAG=$(get_latest_tag) || true
if [[ -z "$LATEST_TAG" ]]; then
  die "无法获取最新版本号"
fi
ok "最新版本: ${C_YELLOW}${LATEST_TAG}${C_RESET}"
log "当前: ${CURRENT}, 最新: ${LATEST_TAG}"
echo ""

# [4/5] 版本对比
step 4 5 "版本对比"
if [[ "$LATEST_TAG" == "$CURRENT" ]]; then
  ok "已是最新版本，无需更新"
  echo ""
  hr
  echo "  ${C_GREEN}✓${C_RESET} 当前已是最新版本 ${LATEST_TAG}"
  hr
  echo ""
  exit 0
fi
ok "需要更新: ${C_GRAY}${CURRENT}${C_RESET} → ${C_CYAN}${LATEST_TAG}${C_RESET}"
echo ""

# 下载资源
step 5 5 "下载并部署"
echo ""

# 获取资产 API URL
BINARY_URL=$(get_asset_url "$LATEST_TAG" "blog-linux-amd64")
SHA_URL=$(get_asset_url "$LATEST_TAG" "blog-linux-amd64.sha256")

if [[ -z "$BINARY_URL" ]] || [[ -z "$SHA_URL" ]]; then
  die "未找到 Release ${LATEST_TAG} 的资产文件"
fi

TMP_BIN=$(mktemp)
TMP_SHA=$(mktemp)
trap 'rm -f "$TMP_BIN" "$TMP_SHA"' EXIT

# 下载二进制
echo "  ${C_GRAY}下载二进制...${C_RESET}"
BIN_SIZE=$(download_file "$BINARY_URL" "$TMP_BIN") || die "二进制下载失败"
sub "blog-linux-amd64" "${C_GREEN}✓${C_RESET} ${BIN_SIZE}"

# 下载 sha256
SHA_SIZE=$(download_file "$SHA_URL" "$TMP_SHA") || die "SHA256 文件下载失败"
sub "blog-linux-amd64.sha256" "${C_GREEN}✓${C_RESET} ${SHA_SIZE}"

# 校验完整性
EXPECTED_SHA=$(awk '{print $1}' "$TMP_SHA")
ACTUAL_SHA=$(sha256sum "$TMP_BIN" | awk '{print $1}')
if [[ "$EXPECTED_SHA" != "$ACTUAL_SHA" ]]; then
  die "SHA256 校验失败 (expected=${EXPECTED_SHA:0:16}... actual=${ACTUAL_SHA:0:16}...)"
fi
sub "SHA256 校验" "${C_GREEN}✓${C_RESET} 通过"
log "SHA256 校验通过"
echo ""

# 部署
echo "  ${C_GRAY}部署服务...${C_RESET}"

# 覆盖安装前备份上一版本二进制（健康检查失败时用于回滚）
if [[ -f "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
  cp "${INSTALL_DIR}/${BINARY_NAME}" "$BACKUP_BIN" || die "备份旧版本二进制失败"
  sub "备份旧版本二进制" "${C_GREEN}✓${C_RESET} ${BACKUP_BIN}"
  log "已备份旧版本二进制: ${BACKUP_BIN}"
else
  sub "备份旧版本二进制" "${C_YELLOW}-${C_RESET} 无旧版本，跳过"
fi

log "停止服务 ${SERVICE_NAME}"
systemctl stop "$SERVICE_NAME" 2>/dev/null || true
sub "停止 ${SERVICE_NAME}" "${C_GREEN}✓${C_RESET}"

cp "$TMP_BIN" "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
echo "$LATEST_TAG" > "${INSTALL_DIR}/version"
sub "替换二进制" "${C_GREEN}✓${C_RESET}"

log "启动服务 ${SERVICE_NAME}"
systemctl start "$SERVICE_NAME"
sub "启动 ${SERVICE_NAME}" "${C_GREEN}✓${C_RESET}"

# 健康检查：systemd 状态 + 本地端口 HTTP 探测（带重试）
sleep 3
HEALTH_OK=0
for ((i = 1; i <= HEALTH_RETRIES; i++)); do
  if check_health; then
    HEALTH_OK=1
    break
  fi
  log "健康检查第 ${i}/${HEALTH_RETRIES} 次未通过"
  sleep 2
done

if [[ "$HEALTH_OK" -eq 1 ]]; then
  sub "健康检查" "${C_GREEN}✓${C_RESET} HTTP 探测通过 (127.0.0.1:${HEALTH_PORT})"
  log "更新成功: ${CURRENT} -> ${LATEST_TAG}"
  echo ""
  hr
  echo "  ${C_GREEN}✓${C_RESET} ${C_BOLD}更新成功${C_RESET}: ${C_GRAY}${CURRENT}${C_RESET} → ${C_CYAN}${LATEST_TAG}${C_RESET}"
  hr
  echo ""
else
  sub "健康检查" "${C_RED}✗${C_RESET} HTTP 探测失败 (127.0.0.1:${HEALTH_PORT})"
  echo ""
  hr
  echo "  ${C_RED}✗${C_RESET} ${C_BOLD}更新失败${C_RESET}: 服务启动或 HTTP 探测失败"
  echo "  ${C_GRAY}排查: journalctl -u ${SERVICE_NAME} -n 20${C_RESET}"
  hr
  echo ""
  log "ERROR: 服务启动或 HTTP 探测失败"

  # 失败路径：存在旧版本备份时自动回滚并重启
  if [[ -f "$BACKUP_BIN" ]]; then
    rollback
  else
    sub "回滚" "${C_YELLOW}-${C_RESET} 无旧版本备份，无法回滚"
    log "无旧版本备份，无法回滚"
  fi
  exit 1
fi
