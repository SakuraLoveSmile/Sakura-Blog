#!/usr/bin/env bash
# 发布前凭据扫描：只扫描 Git 跟踪文件，只输出文件路径和凭据类型，不输出匹配内容。
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
found=0

scan() {
	local label="$1"
	local pattern="$2"
	local path
	while IFS= read -r path; do
		printf '%s: %s\n' "$label" "${path#"$root/"}"
		found=1
	done < <(git -C "$root" grep -IlE "$pattern" -- ':!*.lock' ':!go.sum' ':!*.min.js' ':!*.svg' || true)
}

scan "github-pat" 'gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,}'
scan "cloud-access-key" 'AKIA[0-9A-Z]{16}'
scan "slack-token" 'xox[baprs]-[0-9A-Za-z-]{16,}'

if [[ "$found" -ne 0 ]]; then
	printf '%s\n' "release-secret-check: possible credentials found in tracked files" >&2
	exit 1
fi

printf '%s\n' "release-secret-check: no known credential patterns in tracked files"
