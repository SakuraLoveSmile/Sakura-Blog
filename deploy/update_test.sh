#!/usr/bin/env bash
# update.sh 部署事务测试：不访问 GitHub、不调用真实 systemd，只验证替换/回滚行为。
set -euo pipefail

test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT

fake_bin="$test_root/bin"
mkdir -p "$fake_bin"
cat > "$fake_bin/systemctl" <<'EOF'
#!/usr/bin/env bash
set -u
printf '%s\n' "$*" >> "${TEST_SYSTEMCTL_LOG:?}"
case "${1:-}" in
  is-active) exit "${TEST_SERVICE_ACTIVE:-1}" ;;
  stop) exit "${TEST_STOP_RC:-0}" ;;
  start) exit "${TEST_START_RC:-0}" ;;
  *) exit 0 ;;
esac
EOF
chmod +x "$fake_bin/systemctl"
export PATH="$fake_bin:$PATH"

export INSTALL_DIR="$test_root/install"
export BINARY_NAME=blog
export SERVICE_NAME=blog
export BACKUP_BIN="$INSTALL_DIR/blog.bak"
export LOG_FILE="$test_root/update.log"
export BLOG_HEALTH_INITIAL_DELAY=0
export BLOG_HEALTH_RETRY_DELAY=0
export BLOG_HEALTH_RETRIES=1
export BLOG_ROLLBACK_HEALTH_DELAY=0
export TEST_SYSTEMCTL_LOG="$test_root/systemctl.log"
mkdir -p "$INSTALL_DIR"
: > "$TEST_SYSTEMCTL_LOG"

unset BLOG_PROXY
export http_proxy="http://127.0.0.1:9"
export HTTP_PROXY="$http_proxy"
# shellcheck source=update.sh
source "$(dirname "$0")/update.sh"

fail() {
  printf 'update-test: %s\n' "$1" >&2
  exit 1
}

health_definition="$(declare -f check_health)"
[[ "$health_definition" == *"/readyz"* ]] || fail "health check should use /readyz"

assert_file_content() {
  local path="$1" expected="$2" actual
  actual="$(cat "$path")"
  [[ "$actual" == "$expected" ]] || fail "$path = $actual, want $expected"
}

reset_install() {
  rm -rf "$INSTALL_DIR"
  mkdir -p "$INSTALL_DIR"
  printf 'old-binary\n' > "$INSTALL_DIR/blog"
  chmod +x "$INSTALL_DIR/blog"
  printf 'old-version\n' > "$INSTALL_DIR/version"
  : > "$TEST_SYSTEMCTL_LOG"
  export TEST_SERVICE_ACTIVE=0
  export TEST_STOP_RC=0
  export TEST_START_RC=0
  export TEST_HEALTH_RESULT=ok
}

check_health() {
  [[ "${TEST_HEALTH_RESULT:-ok}" == ok ]]
}

if [[ -n "$PROXY" ]]; then
	fail "BLOG_PROXY should default to direct connection"
fi
if [[ -n "${http_proxy:-}" || -n "${https_proxy:-}" || -n "${HTTP_PROXY:-}" || -n "${HTTPS_PROXY:-}" || -n "${ALL_PROXY:-}" || -n "${all_proxy:-}" ]]; then
	fail "BLOG_PROXY unset should clear inherited proxy variables"
fi

reset_install
new_bin="$test_root/new-success"
printf 'new-binary\n' > "$new_bin"

deploy_binary "$new_bin" "v2" "old-version" || fail "successful deployment returned failure"
assert_file_content "$INSTALL_DIR/blog" "new-binary"
assert_file_content "$INSTALL_DIR/version" "v2"
assert_file_content "$INSTALL_DIR/blog.bak" "old-binary"

reset_install
mkdir "$test_root/not-a-file"
: > "$TEST_SYSTEMCTL_LOG"
if deploy_binary "$test_root/not-a-file" "v2" "old-version"; then
  fail "copy failure should return failure"
fi
assert_file_content "$INSTALL_DIR/blog" "old-binary"
if grep -q '^stop' "$TEST_SYSTEMCTL_LOG"; then
  fail "copy failure stopped the service"
fi

reset_install
export TEST_START_RC=1
if deploy_binary "$new_bin" "v2" "old-version"; then
  fail "start failure should return failure"
fi
assert_file_content "$INSTALL_DIR/blog" "old-binary"
assert_file_content "$INSTALL_DIR/version" "old-version"

reset_install
export TEST_HEALTH_RESULT=fail
if deploy_binary "$new_bin" "v2" "old-version"; then
  fail "health failure should return failure"
fi
assert_file_content "$INSTALL_DIR/blog" "old-binary"
assert_file_content "$INSTALL_DIR/version" "old-version"
python3 -c "
import json, sys
data = json.load(open('$STATUS_FILE'))
if data.get('state') != 'failed':
    sys.exit('state should be failed')
if not data.get('rolled_back'):
    sys.exit('rolled_back should be true')
" || fail "status.json failed to record rollback state"

reset_install
deploy_binary "$new_bin" "v2" "old-version" || fail "successful deployment returned failure"
python3 -c "
import json, sys
data = json.load(open('$STATUS_FILE'))
if data.get('state') != 'succeeded':
    sys.exit('state should be succeeded')
if data.get('stage') != 'succeeded':
    sys.exit('stage should be succeeded')
if not data.get('completed_at'):
    sys.exit('completed_at should be present')
" || fail "status.json failed to record succeeded state"

rm -f "$INSTALL_DIR/blog"
: > "$TEST_SYSTEMCTL_LOG"
if deploy_binary "$new_bin" "v2" "unknown"; then
  fail "first install should be rejected"
fi
if grep -q '^stop' "$TEST_SYSTEMCTL_LOG"; then
  fail "first install stopped the service"
fi

# --- 测试 consume_oldest_request ---
export REQUESTS_DIR="$test_root/requests"
mkdir -p "$REQUESTS_DIR"

# 1. 正常请求消费
printf '{"version":1,"task_id":"task-test-42","target_tag":"v1.5.0","created_at":"2026-08-15T10:00:00Z"}\n' > "$REQUESTS_DIR/task-test-42.json"
consume_oldest_request || fail "consume_oldest_request failed on valid request"
[[ "${TASK_ID:-}" == "task-test-42" ]] || fail "TASK_ID = ${TASK_ID:-}, want task-test-42"
[[ "${TARGET_TAG:-}" == "v1.5.0" ]] || fail "TARGET_TAG = ${TARGET_TAG:-}, want v1.5.0"
[[ ! -f "$REQUESTS_DIR/task-test-42.json" ]] || fail "request file was not removed after consumption"

# 2. 符号链接请求应被拒绝并清理，绝不被消费
secret_target="$test_root/secret.txt"
printf 'sensitive_data\n' > "$secret_target"
ln -s "$secret_target" "$REQUESTS_DIR/symlink.json"
unset TASK_ID TARGET_TAG
if consume_oldest_request; then
  fail "consume_oldest_request should reject symlink"
fi
[[ ! -L "$REQUESTS_DIR/symlink.json" ]] || fail "symlink should be unlinked"
assert_file_content "$secret_target" "sensitive_data"

# 3. 超大文件 (>64KB) 应被拒绝
python3 -c "print('x' * 70000)" > "$REQUESTS_DIR/oversized.json"
if consume_oldest_request; then
  fail "consume_oldest_request should reject oversized file"
fi

# --- 测试 acquire_lock 排他锁 ---
export LOCK_FILE="$test_root/update.lock"
acquire_lock || fail "first acquire_lock should succeed"

# 在子进程中再次尝试获取同一把锁，必须失败
if bash -c '
source "'"$PWD"'/deploy/update.sh"
export LOCK_FILE="'"$LOCK_FILE"'"
if acquire_lock; then
  exit 0
else
  exit 1
fi
'; then
  fail "concurrent acquire_lock in subshell should have failed"
fi

release_lock

printf '%s\n' "update-test: all deployment and security cases passed"
