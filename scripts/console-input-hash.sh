#!/usr/bin/env bash

set -euo pipefail

root_dir="${CONSOLE_INPUT_ROOT:-$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)}"
mode="${1:-}"

if [[ "$mode" != "" && "$mode" != "--lock" ]] || [[ "$#" -gt 1 ]]; then
  echo "usage: $0 [--lock]" >&2
  exit 2
fi

hash_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

hash_stream() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | awk '{print $1}'
  else
    shasum -a 256 | awk '{print $1}'
  fi
}

if [[ "$mode" == "--lock" ]]; then
  lockfile="$root_dir/pnpm-lock.yaml"
  [[ -f "$lockfile" ]] || { echo "console-input-hash: missing $lockfile" >&2; exit 1; }
  hash_file "$lockfile"
  exit 0
fi

manifest="$(mktemp)"
trap 'rm -f "$manifest"' EXIT

add_file() {
  local absolute="$1"
  if [[ -f "$absolute" ]]; then
    printf '%s\n' "${absolute#"$root_dir"/}" >> "$manifest"
  fi
}

if [[ -d "$root_dir/modules/console/web/src" ]]; then
  while IFS= read -r path; do
    add_file "$path"
  done < <(find "$root_dir/modules/console/web/src" -type f -print)
fi

for path in "$root_dir/modules/console/web"/*.ts "$root_dir/modules/console/web"/*.json; do
  add_file "$path"
done

add_file "$root_dir/package.json"
add_file "$root_dir/pnpm-lock.yaml"
add_file "$root_dir/pnpm-workspace.yaml"

if [[ -d "$root_dir/packages" ]]; then
  while IFS= read -r path; do
    add_file "$path"
  done < <(
    find "$root_dir/packages" \
      -type d -name node_modules -prune -o \
      -type f \( -path '*/src/*' -o -name package.json \) -print
  )
fi

sort -u "$manifest" -o "$manifest"
{
  while IFS= read -r relative; do
    printf '%s\t%s\n' "$relative" "$(hash_file "$root_dir/$relative")"
  done < "$manifest"
} | hash_stream
