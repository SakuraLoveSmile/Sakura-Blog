#!/usr/bin/env bash
# 发布前历史凭据扫描：扫描所有本地 Git refs，只输出 commit/path/type，不输出匹配内容。
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
found=0

labels=("github-pat" "cloud-access-key" "slack-token" "private-key")
patterns=(
	'gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,}'
	'AKIA[0-9A-Z]{16}'
	'xox[baprs]-[0-9A-Za-z-]{16,}'
	'-----BEGIN (RSA|EC|OPENSSH|DSA|PGP) PRIVATE KEY-----'
)
combined='gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16}|xox[baprs]-[0-9A-Za-z-]{16,}|-----BEGIN (RSA|EC|OPENSSH|DSA|PGP) PRIVATE KEY-----'

while IFS= read -r commit; do
	while IFS= read -r path; do
		content="$(git -C "$root" show "$commit:$path" 2>/dev/null || true)"
		for i in "${!patterns[@]}"; do
			if printf '%s' "$content" | grep -Eq -- "${patterns[$i]}"; then
				printf '%s: %s: %s\n' "${labels[$i]}" "$commit" "${path#"$root/"}"
				found=1
				break
			fi
		done
	done < <(
		git -C "$root" grep -IlE -e "$combined" "$commit" -- \
			':!*.lock' ':!go.sum' ':!*.min.js' ':!*.svg' ':!*.map' \
			|| true
	)
	while IFS= read -r path; do
		content="$(git -C "$root" show "$commit:$path" 2>/dev/null || true)"
		if printf '%s' "$content" | awk '
			BEGIN { found = 0; IGNORECASE = 1 }
			/^[[:space:]]*(password|secret|secret_key|api_key|token|github_token|access_key|private_key)[[:space:]]*:/ {
				value = $0
				sub(/^[^:]*:[[:space:]]*/, "", value)
				gsub(/[[:space:]"'"'"'`]/, "", value)
				if (value ~ /^#/) value = ""
				if (value != "" && value != "null" && value != "none" && value !~ /^(example|changeme|replace|your)/) found = 1
			}
			/^[A-Za-z_]*(PASSWORD|SECRET|TOKEN|PRIVATE_KEY)[A-Za-z_]*=/ {
				value = $0
				sub(/^[^=]*=/, "", value)
				gsub(/[[:space:]"'"'"'`]/, "", value)
				if (value != "" && value !~ /^(example|changeme|replace|your)/) found = 1
			}
			END { exit(found ? 0 : 1) }
		'; then
			printf '%s: %s: %s\n' "secret-config-value" "$commit" "${path#"$root/"}"
			found=1
		fi
	done < <(
		git -C "$root" ls-tree -r --name-only "$commit" | rg '(^|/)(config/(app|site)\.yml|\.env($|\.)|.*(secret|credential|password).*\.(yml|yaml|json|env))$' || true
	)
done < <(git -C "$root" rev-list --all)

if [[ "$found" -ne 0 ]]; then
	printf '%s\n' "release-secret-history-check: possible credentials found in Git history" >&2
	printf '%s\n' "release-secret-history-check: rotate/revoke confirmed credentials before release; this scan does not revoke anything" >&2
	exit 1
fi

printf '%s\n' "release-secret-history-check: no known credential patterns in Git history"
