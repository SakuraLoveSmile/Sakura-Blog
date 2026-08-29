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

rm -f "$INSTALL_DIR/blog"
: > "$TEST_SYSTEMCTL_LOG"
if deploy_binary "$new_bin" "v2" "unknown"; then
  fail "first install should be rejected"
fi
if grep -q '^stop' "$TEST_SYSTEMCTL_LOG"; then
  fail "first install stopped the service"
fi

printf '%s\n' "update-test: all deployment cases passed"
