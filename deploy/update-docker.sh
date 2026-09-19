#!/usr/bin/env bash
# 博客自动更新脚本（Docker 容器部署版）：检查 GitHub Release 最新版本，下载并替换二进制，重启容器。
# 与 update.sh 同一契约：消费 update/requests 请求、写 status.json / update.log、失败自动回滚。
# 覆盖安装前备份上一版本二进制；健康检查（本地端口 HTTP 探测）失败时自动回滚并重启。
# 用法：
#   手动执行：sudo /opt/blog/update-docker.sh
#   自动执行：配合 blog-update-docker.path / cron 监听 update/requests/
#
# 依赖：curl, sha256sum, docker, python3
# 前提：安装目录以「目录 bind mount」挂入容器（如 /opt/blog -> /app），
#       容器启动命令执行 <root>/blog 二进制；不要以单文件方式挂载二进制。

set -euo pipefail

# ---------- 配置 ----------
REPO="${BLOG_REPO:-SakuraLoveSmile/Sakura-Blog}"
INSTALL_DIR="${INSTALL_DIR:-/opt/blog}"
BINARY_NAME="${BINARY_NAME:-blog}"
# 容器名（docker ps 的 Names 列）
CONTAINER="${BLOG_CONTAINER:-blog}"
# 请求与状态控制目录（与应用内 updaterunner 共用契约）
CONTROL_DIR="${BLOG_CONTROL_DIR:-${INSTALL_DIR}/update}"
REQUESTS_DIR="${BLOG_REQUESTS_DIR:-${CONTROL_DIR}/requests}"
STATUS_FILE="${BLOG_STATUS_FILE:-${CONTROL_DIR}/status.json}"
LOCK_FILE="${BLOG_LOCK_FILE:-${CONTROL_DIR}/update.lock}"
# GitHub API（私有仓库需 Token 认证）
API_BASE="${BLOG_API_BASE:-https://api.github.com}"
# 私有仓库 Release 下载需要 PAT（权限 repo），写入 /opt/blog/.token（chmod 600）
TOKEN_FILE="${TOKEN_FILE:-${INSTALL_DIR}/.token}"
# 日志文件
LOG_FILE="${LOG_FILE:-${INSTALL_DIR}/data/update.log}"
# 回滚备份：覆盖安装前备份上一版本二进制，健康检查失败时还原
BACKUP_BIN="${BACKUP_BIN:-${INSTALL_DIR}/${BINARY_NAME}.bak}"
# 健康检查：对宿主映射端口做 HTTP 探测（与容器发布端口一致，如 8081->8080 则填 8081）
HEALTH_PORT="${BLOG_HEALTH_PORT:-8080}"
HEALTH_RETRIES="${BLOG_HEALTH_RETRIES:-5}"
HEALTH_INITIAL_DELAY="${BLOG_HEALTH_INITIAL_DELAY:-3}"
HEALTH_RETRY_DELAY="${BLOG_HEALTH_RETRY_DELAY:-2}"
ROLLBACK_HEALTH_DELAY="${BLOG_ROLLBACK_HEALTH_DELAY:-3}"
# HTTP 代理（国内服务器走代理加速 GitHub，留空则直连）
PROXY="${BLOG_PROXY:-}"
unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY all_proxy
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

# 进程退出统一清理
cleanup() {
  rm -f "${TMP_BIN:-}" "${TMP_SHA:-}"
}

# 状态机持久化：将当前阶段原子同步落盘至 status.json，对外暴露 0644 供只读查询
write_status() {
  local state="$1" stage="$2" msg="$3" err="${4:-}" rolled_back="${5:-false}" rollback_ok="${6:-false}"
  local now
  now=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
  mkdir -p "$(dirname "$STATUS_FILE")"
  python3 -c "
import json, os, sys

status_path = sys.argv[1]
task_id = sys.argv[2]
state = sys.argv[3]
stage = sys.argv[4]
current_v = sys.argv[5]
target_v = sys.argv[6]
now = sys.argv[7]
msg = sys.argv[8]
err_msg = sys.argv[9]
rolled_back = sys.argv[10].lower() == 'true'
rollback_ok = sys.argv[11].lower() == 'true'

existing = {}
if os.path.isfile(status_path):
    try:
        with open(status_path, 'r', encoding='utf-8') as f:
            existing = json.load(f)
    except Exception:
        existing = {}

if not task_id:
    task_id = existing.get('task_id', '')
if not target_v:
    target_v = existing.get('target_version', '')
started_at = existing.get('started_at') or now

doc = {
    'task_id': task_id,
    'state': state,
    'stage': stage,
    'current_version': current_v,
    'target_version': target_v,
    'started_at': started_at,
    'updated_at': now,
    'message': msg,
    'error': err_msg,
    'rolled_back': rolled_back,
    'rollback_succeeded': rollback_ok,
}
if state in ('succeeded', 'failed'):
    doc['completed_at'] = now

tmp = status_path + '.tmp'
try:
    with open(tmp, 'w', encoding='utf-8') as f:
        json.dump(doc, f, indent=2, ensure_ascii=False)
        f.write('\n')
        f.flush()
        os.fsync(f.fileno())
    os.chmod(tmp, 0o644)
    os.replace(tmp, status_path)
except Exception as e:
    sys.stderr.write(f'write_status failed: {e}\n')
" "$STATUS_FILE" "${TASK_ID:-}" "$state" "$stage" "${CURRENT:-unknown}" "${TARGET_TAG:-}" "$now" "$msg" "$err" "$rolled_back" "$rollback_ok" 2>/dev/null || true
}

# 消费请求目录中最早的合法更新请求（Python3 防 TOCTOU / 符号链接 / 超大文件）
consume_oldest_request() {
  if [[ ! -d "$REQUESTS_DIR" ]]; then
    return 1
  fi
  local req_out
  req_out=$(python3 -c "
import os, sys, stat, json, re

req_dir = sys.argv[1]
if not os.path.isdir(req_dir):
    sys.exit(1)

entries = sorted(os.listdir(req_dir))
valid_task_id = re.compile(r'^[a-zA-Z0-9_-]{1,64}$')
valid_tag = re.compile(r'^[a-zA-Z0-9._+-]{1,64}$')

for name in entries:
    if name.startswith('.') or not name.endswith('.json'):
        continue
    path = os.path.join(req_dir, name)
    flags = os.O_RDONLY
    if hasattr(os, 'O_NOFOLLOW'):
        flags |= os.O_NOFOLLOW
    try:
        fd = os.open(path, flags)
    except Exception:
        try:
            if os.path.islink(path):
                os.unlink(path)
        except Exception:
            pass
        continue
    try:
        st = os.fstat(fd)
        if not stat.S_ISREG(st.st_mode):
            try: os.unlink(path)
            except Exception: pass
            continue
        if st.st_size > 65536:
            try: os.unlink(path)
            except Exception: pass
            continue
        content = os.read(fd, 65536)
    finally:
        os.close(fd)

    try:
        data = json.loads(content.decode('utf-8'))
        task_id = str(data.get('task_id', '')).strip()
        target_tag = str(data.get('target_tag', '')).strip()
        version = data.get('version', 1)
        if version != 1:
            try: os.unlink(path)
            except Exception: pass
            continue
        if not valid_task_id.match(task_id) or not valid_tag.match(target_tag):
            try: os.unlink(path)
            except Exception: pass
            continue

        try: os.unlink(path)
        except Exception: pass

        print(f'TASK_ID={task_id}')
        print(f'TARGET_TAG={target_tag}')
        sys.exit(0)
    except Exception:
        try: os.unlink(path)
        except Exception: pass
        continue

sys.exit(1)
" "$REQUESTS_DIR" 2>/dev/null || true)

  if [[ -n "$req_out" ]]; then
    eval "$req_out"
    return 0
  fi
  return 1
}

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

# 从 Release 获取指定资产的 API URL（兼容带或不带 v 前缀）
get_asset_url() {
  local tag="$1" filename="$2"
  local tags_to_try=("$tag")
  if [[ "$tag" != v* ]]; then
    tags_to_try=("$tag" "v$tag")
  else
    tags_to_try=("$tag" "${tag#v}")
  fi

  for try_tag in "${tags_to_try[@]}"; do
    local tmp_response
    tmp_response=$(mktemp)
    local asset_url=""

    if [[ -n "$AUTH_HEADER" ]]; then
      curl -sfL --max-time 30 -H "$AUTH_HEADER" \
        "${API_BASE}/repos/${REPO}/releases/tags/${try_tag}" > "$tmp_response" 2>/dev/null || true
    else
      curl -sfL --max-time 30 \
        "${API_BASE}/repos/${REPO}/releases/tags/${try_tag}" > "$tmp_response" 2>/dev/null || true
    fi

    asset_url=$(python3 -c "
import json, sys
try:
    with open('$tmp_response', 'r', encoding='utf-8') as f:
        data = json.load(f)
    for a in data.get('assets', []):
        if a.get('name') == sys.argv[1]:
            print(a.get('url', ''))
            break
except Exception:
    pass
" "$filename" 2>/dev/null)

    rm -f "$tmp_response"
    if [[ -n "$asset_url" ]]; then
      echo "$asset_url"
      return 0
    fi
  done

  echo ""
  return 1
}

# 健康检查：容器处于 running 状态，且本地 readiness 端点返回 2xx/3xx
check_health() {
  local running
  running=$(docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null || echo "false")
  [[ "$running" == "true" ]] || return 1
  local code=""
  code=$(curl -s -o /dev/null -w "%{http_code}" --noproxy '*' \
    --max-time 5 "http://127.0.0.1:${HEALTH_PORT}/readyz" 2>/dev/null || echo "000")
  [[ "$code" =~ ^(200|301|302|303|307|308)$ ]]
}

# 回滚：还原备份的上一版本二进制与版本号，并重启容器
rollback() {
  write_status "running" "rolling_back" "健康检查失败，正在回滚到 ${CURRENT}"
  echo "  ${C_YELLOW}↩ 健康检查失败，回滚到 ${CURRENT} ...${C_RESET}"
  log "健康检查失败，开始回滚: ${LATEST_TAG} -> ${CURRENT}"
  if ! cp "$BACKUP_BIN" "${INSTALL_DIR}/${BINARY_NAME}" || ! chmod +x "${INSTALL_DIR}/${BINARY_NAME}"; then
    sub "还原旧版本二进制" "${C_RED}✗${C_RESET} 无法还原备份"
    log "ERROR: 回滚失败，无法还原备份二进制"
    write_status "failed" "failed" "回滚失败：无法还原备份二进制" "回滚失败：无法还原备份二进制" "true" "false"
    return 1
  fi
  if ! printf '%s\n' "$CURRENT" > "${INSTALL_DIR}/version"; then
    sub "还原版本号" "${C_RED}✗${C_RESET} 无法写入 ${INSTALL_DIR}/version"
    log "ERROR: 回滚无法写入版本号"
    write_status "failed" "failed" "回滚失败：无法写入版本号" "回滚失败：无法写入版本号" "true" "false"
    return 1
  fi
  sub "还原旧版本二进制" "${C_GREEN}✓${C_RESET}"

  if ! docker restart "$CONTAINER" >/dev/null 2>&1; then
    sub "重启旧版本容器" "${C_RED}✗${C_RESET} 启动失败，需人工介入"
    log "ERROR: 回滚后旧容器启动失败"
    write_status "failed" "failed" "回滚失败：旧容器启动失败" "回滚失败：旧容器启动失败" "true" "false"
    return 1
  fi
  if ! sleep "$ROLLBACK_HEALTH_DELAY"; then
    sub "回滚后健康检查" "${C_RED}✗${C_RESET} 等待失败，需人工介入"
    log "ERROR: 回滚后健康检查等待失败"
    write_status "failed" "failed" "回滚健康检查等待失败" "回滚健康检查等待失败" "true" "false"
    return 1
  fi
  if check_health; then
    sub "回滚后健康检查" "${C_GREEN}✓${C_RESET} 已恢复 ${CURRENT}"
    log "回滚成功: ${LATEST_TAG} -> ${CURRENT}"
    write_status "failed" "failed" "健康检查失败，已回滚到 ${CURRENT}" "新版本服务未通过健康检查" "true" "true"
    return 0
  fi
  sub "回滚后健康检查" "${C_RED}✗${C_RESET} 服务仍未恢复，需人工介入"
  log "ERROR: 回滚后服务仍未恢复"
  write_status "failed" "failed" "回滚失败：服务仍未恢复" "回滚后健康检查未通过" "true" "false"
  return 1
}

# deploy_binary 将已校验的新二进制 staged 后再切换，并保证部署阶段任一失败都回滚。
# docker 模式按目录 bind mount 前提直接替换文件（运行中进程持有旧 inode，安全），
# 再 docker restart 让容器执行新二进制。
# 该函数只接受已有旧版本的升级；首次安装由部署文档中的安装流程负责。
deploy_binary() {
  local tmp_bin="$1"
  local latest_tag="$2"
  local current_version="$3"
  local staged_bin="${INSTALL_DIR}/${BINARY_NAME}.new"
  local staged_version="${INSTALL_DIR}/version.new"

  CURRENT="$current_version"
  LATEST_TAG="$latest_tag"

  if [[ ! -f "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
    log "ERROR: 无旧版本二进制，拒绝使用 update-docker.sh 首次安装"
    sub "旧版本检查" "${C_RED}✗${C_RESET} 未找到 ${INSTALL_DIR}/${BINARY_NAME}"
    write_status "failed" "replacing" "未找到旧版本二进制" "无旧版本二进制，拒绝使用 update-docker.sh 首次安装"
    return 1
  fi

  write_status "running" "replacing" "正在备份并替换二进制"

  rm -f "$staged_bin" "$staged_version"
  # 先直接复制当前运行版本；失败时绝不动服务。
  if ! cp "${INSTALL_DIR}/${BINARY_NAME}" "$BACKUP_BIN"; then
    log "ERROR: 备份旧版本二进制失败"
    write_status "failed" "replacing" "备份旧版本二进制失败" "备份旧版本二进制失败"
    return 1
  fi

  if ! cp "$tmp_bin" "$staged_bin"; then
    rm -f "$staged_bin" "$staged_version"
    log "ERROR: staged 二进制复制失败"
    write_status "failed" "replacing" "staged 二进制复制失败" "staged 二进制复制失败"
    return 1
  fi
  if ! chmod +x "$staged_bin"; then
    rm -f "$staged_bin" "$staged_version"
    log "ERROR: staged 二进制权限设置失败"
    write_status "failed" "replacing" "staged 二进制权限设置失败" "staged 二进制权限设置失败"
    return 1
  fi
  if ! printf '%s\n' "$latest_tag" > "$staged_version"; then
    rm -f "$staged_bin" "$staged_version"
    log "ERROR: staged 版本号写入失败"
    write_status "failed" "replacing" "staged 版本号写入失败" "staged 版本号写入失败"
    return 1
  fi

  sub "备份旧版本二进制" "${C_GREEN}✓${C_RESET} ${BACKUP_BIN}"
  log "已备份旧版本二进制: ${BACKUP_BIN}"

  if ! mv "$staged_bin" "${INSTALL_DIR}/${BINARY_NAME}"; then
    rm -f "$staged_version"
    rollback || true
    return 1
  fi
  if ! mv "$staged_version" "${INSTALL_DIR}/version"; then
    rm -f "$staged_version"
    rollback || true
    return 1
  fi
  sub "原子替换二进制" "${C_GREEN}✓${C_RESET}"

  log "重启容器 ${CONTAINER}"
  if ! docker restart "$CONTAINER" >/dev/null 2>&1; then
    sub "重启容器 ${CONTAINER}" "${C_RED}✗${C_RESET} 重启失败"
    log "ERROR: 容器重启失败"
    rollback || true
    return 1
  fi
  sub "重启容器 ${CONTAINER}" "${C_GREEN}✓${C_RESET}"

  write_status "running" "health_checking" "正在进行健康检查"

  sleep "$HEALTH_INITIAL_DELAY"
  local health_ok=0
  local i
  for ((i = 1; i <= HEALTH_RETRIES; i++)); do
    if check_health; then
      health_ok=1
      break
    fi
    log "健康检查第 ${i}/${HEALTH_RETRIES} 次未通过"
    sleep "$HEALTH_RETRY_DELAY"
  done

  if [[ "$health_ok" -eq 1 ]]; then
    sub "健康检查" "${C_GREEN}✓${C_RESET} HTTP 探测通过 (127.0.0.1:${HEALTH_PORT})"
    rm -f "$staged_bin" "$staged_version"
    write_status "succeeded" "succeeded" "更新成功: ${CURRENT} -> ${latest_tag}"
    return 0
  fi

  sub "健康检查" "${C_RED}✗${C_RESET} HTTP 探测失败 (127.0.0.1:${HEALTH_PORT})"
  log "ERROR: 服务启动或 HTTP 探测失败"
  rollback || true
  rm -f "$staged_bin" "$staged_version"
  return 1
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

# 与恢复任务共享进程级锁；锁跨博客重启保留在更新器进程中。
# Python 已是更新脚本依赖，O_NOFOLLOW 防止以 root 身份跟随可写数据目录内的链接。
run_with_maintenance_lock() {
  python3 - "$0" "$@" <<'PYLOCK'
import fcntl
import glob
import json
import os
import subprocess
import sys
import pwd
import stat
import time

install = os.environ.get("INSTALL_DIR", "/opt/blog")
db_path = os.environ.get("BLOG_DB", os.path.join(install, "data", "blog.db"))
lock_path = os.environ.get("BLOG_MAINTENANCE_LOCK", os.path.join(os.path.dirname(db_path), "maintenance.lock"))
control = os.environ.get("BLOG_CONTROL_DIR", os.path.join(install, "update"))
update_lock_path = os.environ.get("BLOG_LOCK_FILE", os.path.join(control, "update.lock"))

def open_lock(path, service_owner=False):
    os.makedirs(os.path.dirname(path), mode=0o700, exist_ok=True)
    try:
        fd = os.open(path, os.O_CREAT | os.O_EXCL | os.O_RDWR | os.O_NOFOLLOW, 0o600)
        if service_owner and os.geteuid() == 0:
            # docker 部署宿主通常无 blog 用户（进程以容器用户运行），找不到时跳过属主调整
            try:
                owner = pwd.getpwnam("blog")
                os.fchown(fd, owner.pw_uid, owner.pw_gid)
            except KeyError:
                pass
    except FileExistsError:
        fd = os.open(path, os.O_RDWR | os.O_NOFOLLOW)
    info = os.fstat(fd)
    if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_size != 0:
        os.close(fd)
        raise OSError("invalid operation lock")
    return fd

def wait_lock(fd):
    try:
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        print("等待恢复或更新锁，更新请求尚未消费", file=sys.stderr, flush=True)
        fcntl.flock(fd, fcntl.LOCK_EX)

try:
    fd = open_lock(lock_path, service_owner=True)
    update_fd = open_lock(update_lock_path)
    if os.fstat(fd).st_ino == os.fstat(update_fd).st_ino and os.fstat(fd).st_dev == os.fstat(update_fd).st_dev:
        raise OSError("maintenance and update locks must be distinct")
    # 固定顺序：维护锁 -> 更新锁 -> 检查恢复日志 -> 消费请求。
    wait_lock(fd)
    wait_lock(update_fd)
    for name in glob.glob(os.path.join(os.path.dirname(db_path), "backups", ".restore-*.json")):
        source = os.open(name, os.O_RDONLY | os.O_NOFOLLOW)
        with os.fdopen(source, "rb") as handle:
            raw = handle.read(1048577)
        if len(raw) > 1048576:
            raise ValueError("oversized restore journal")
        task = json.loads(raw)
        if not isinstance(task, dict) or task.get("version") != 1 or task.get("status") not in ("succeeded", "rolled_back"):
            raise ValueError("unfinished or unsupported restore task")
except (OSError, ValueError, TypeError, KeyError):
    print("更新被阻止：锁文件无效或存在未完成/不支持的恢复日志；请求保留", file=sys.stderr, flush=True)
    sys.exit(75)
environment = dict(os.environ)
environment["BLOG_MAINTENANCE_LOCK_HELD"] = "1"
result = subprocess.run(["bash", sys.argv[1], *sys.argv[2:]], env=environment, pass_fds=(fd, update_fd))
sys.exit(result.returncode)
PYLOCK
}

# ---------- 主流程 ----------
main() {
  mkdir -p "$(dirname "$LOG_FILE")"
  mkdir -p "$CONTROL_DIR"

  trap cleanup EXIT INT TERM

  log "=== 开始检查更新（docker 驱动） ==="

  if ! command -v docker >/dev/null 2>&1; then
    die "未找到 docker 命令；update-docker.sh 需在宿主环境运行"
  fi

  # 读取 Token
  TOKEN=$(get_token)
  AUTH_HEADER=""
  [[ -n "$TOKEN" ]] && AUTH_HEADER="Authorization: token ${TOKEN}"

  CURRENT=$(get_current_version)

  TASK_ID=""
  TARGET_TAG=""

  if consume_oldest_request; then
    log "收到更新请求: task_id=${TASK_ID}, target_tag=${TARGET_TAG}"
    write_status "running" "queued" "更新任务已就绪"
  else
    # 头部信息
    echo ""
    hr
    echo "  ${C_BOLD}${C_CYAN}Blog 自动更新（docker）${C_RESET}"
    echo "  ${C_GRAY}仓库${C_RESET}    ${REPO}"
    echo "  ${C_GRAY}容器${C_RESET}    ${CONTAINER}"
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
    ok "当前版本: ${C_YELLOW}${CURRENT}${C_RESET}"
    echo ""

    # [3/5] 查询最新版本
    step 3 5 "查询 GitHub 最新版本"
    TARGET_TAG=$(get_latest_tag) || true
    if [[ -z "$TARGET_TAG" ]]; then
      die "无法获取最新版本号"
    fi
    ok "最新版本: ${C_YELLOW}${TARGET_TAG}${C_RESET}"
    log "当前: ${CURRENT}, 最新: ${TARGET_TAG}"
    echo ""

    # [4/5] 版本对比
    step 4 5 "版本对比"
    if [[ "$TARGET_TAG" == "$CURRENT" || "v$TARGET_TAG" == "$CURRENT" || "$TARGET_TAG" == "v$CURRENT" ]]; then
      ok "已是最新版本，无需更新"
      echo ""
      hr
      echo "  ${C_GREEN}✓${C_RESET} 当前已是最新版本 ${TARGET_TAG}"
      hr
      echo ""
      exit 0
    fi
    ok "需要更新: ${C_GRAY}${CURRENT}${C_RESET} → ${C_CYAN}${TARGET_TAG}${C_RESET}"
    echo ""
  fi

  if [[ ! -f "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
    write_status "failed" "replacing" "未找到旧版本二进制" "未找到旧版本二进制；update-docker.sh 只用于升级"
    die "未找到旧版本二进制；update-docker.sh 只用于升级，首次安装请按 deploy/README.md 执行"
  fi

  # [5/5] 下载资源并部署
  step 5 5 "下载并部署 (${TARGET_TAG})"
  echo ""

  write_status "running" "downloading" "正在下载资产"

  # 获取资产 API URL
  BINARY_URL=$(get_asset_url "$TARGET_TAG" "blog-linux-amd64") || true
  SHA_URL=$(get_asset_url "$TARGET_TAG" "blog-linux-amd64.sha256") || true

  if [[ -z "$BINARY_URL" ]] || [[ -z "$SHA_URL" ]]; then
    write_status "failed" "downloading" "未找到 Release 资产文件" "未找到 Release ${TARGET_TAG} 的资产文件"
    die "未找到 Release ${TARGET_TAG} 的资产文件"
  fi

  TMP_BIN=$(mktemp)
  TMP_SHA=$(mktemp)

  # 下载二进制
  echo "  ${C_GRAY}下载二进制...${C_RESET}"
  if ! BIN_SIZE=$(download_file "$BINARY_URL" "$TMP_BIN"); then
    write_status "failed" "downloading" "二进制下载失败" "二进制下载失败"
    die "二进制下载失败"
  fi
  sub "blog-linux-amd64" "${C_GREEN}✓${C_RESET} ${BIN_SIZE}"

  # 下载 sha256
  if ! SHA_SIZE=$(download_file "$SHA_URL" "$TMP_SHA"); then
    write_status "failed" "downloading" "SHA256 文件下载失败" "SHA256 文件下载失败"
    die "SHA256 文件下载失败"
  fi
  sub "blog-linux-amd64.sha256" "${C_GREEN}✓${C_RESET} ${SHA_SIZE}"

  # 校验完整性
  write_status "running" "verifying" "正在校验 SHA256 完整性"
  EXPECTED_SHA=$(awk '{print $1}' "$TMP_SHA")
  ACTUAL_SHA=$(sha256sum "$TMP_BIN" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$TMP_BIN" 2>/dev/null | awk '{print $1}')
  if [[ "$EXPECTED_SHA" != "$ACTUAL_SHA" ]]; then
    write_status "failed" "verifying" "SHA256 校验失败" "expected=${EXPECTED_SHA:0:16}... actual=${ACTUAL_SHA:0:16}..."
    die "SHA256 校验失败 (expected=${EXPECTED_SHA:0:16}... actual=${ACTUAL_SHA:0:16}...)"
  fi
  sub "SHA256 校验" "${C_GREEN}✓${C_RESET} 通过"
  log "SHA256 校验通过"
  echo ""

  # 部署
  echo "  ${C_GRAY}部署服务...${C_RESET}"
  if deploy_binary "$TMP_BIN" "$TARGET_TAG" "$CURRENT"; then
    log "更新成功: ${CURRENT} -> ${TARGET_TAG}"
    echo ""
    hr
    echo "  ${C_GREEN}✓${C_RESET} ${C_BOLD}更新成功${C_RESET}: ${C_GRAY}${CURRENT}${C_RESET} → ${C_CYAN}${TARGET_TAG}${C_RESET}"
    hr
    echo ""
  else
    echo ""
    hr
    echo "  ${C_RED}✗${C_RESET} ${C_BOLD}更新失败${C_RESET}: 服务启动或 HTTP 探测失败"
    echo "  ${C_GRAY}排查: docker logs --tail 20 ${CONTAINER}${C_RESET}"
    hr
    echo ""
    log "ERROR: 服务启动或 HTTP 探测失败"
    exit 1
  fi
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  if [[ "${BLOG_MAINTENANCE_LOCK_HELD:-}" != "1" ]]; then
    run_with_maintenance_lock "$@"
    exit $?
  fi
  main "$@"
fi
