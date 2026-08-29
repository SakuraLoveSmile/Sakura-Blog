#!/usr/bin/env bash

set -euo pipefail

head_ref="${HEAD_REF:-}"
head_repo="${HEAD_REPO:-}"
base_repo="${BASE_REPO:-}"

if [[ "$head_ref" != "main" ]]; then
  echo "release-source: PR head ref must be main (got: ${head_ref:-<empty>})" >&2
  exit 1
fi

if [[ -z "$head_repo" || -z "$base_repo" || "$head_repo" != "$base_repo" ]]; then
  echo "release-source: PR head repository must equal base repository" >&2
  exit 1
fi

echo "release-source: verified ${head_repo}:main -> ${base_repo}"
