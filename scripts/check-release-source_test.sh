#!/usr/bin/env bash

set -euo pipefail

script="$(dirname "$0")/check-release-source.sh"

expect_success() {
  if ! HEAD_REF="$1" HEAD_REPO="$2" BASE_REPO="$3" bash "$script" >/dev/null; then
    echo "release-source-test: expected success for $1 $2 $3" >&2
    exit 1
  fi
}

expect_failure() {
  if HEAD_REF="$1" HEAD_REPO="$2" BASE_REPO="$3" bash "$script" >/dev/null 2>&1; then
    echo "release-source-test: expected failure for $1 $2 $3" >&2
    exit 1
  fi
}

expect_success "main" "SakuraLoveSmile/Sakura-Blog" "SakuraLoveSmile/Sakura-Blog"
expect_failure "main" "attacker/Sakura-Blog" "SakuraLoveSmile/Sakura-Blog"
expect_failure "dev" "SakuraLoveSmile/Sakura-Blog" "SakuraLoveSmile/Sakura-Blog"
expect_failure "main" "" "SakuraLoveSmile/Sakura-Blog"

printf '%s\n' "release-source-test: all cases passed"
