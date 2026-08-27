#!/bin/sh
set -eu

fail=0

if rg -n '"blog/modules/(console|stellar)' internal --glob '*.go'; then
	printf '%s\n' 'module-check: internal must not import business modules' >&2
	fail=1
fi
if rg -n '"blog/modules/stellar' modules/console --glob '*.go'; then
	printf '%s\n' 'module-check: console must not import stellar' >&2
	fail=1
fi
if rg -n '"blog/modules/console' modules/stellar --glob '*.go'; then
	printf '%s\n' 'module-check: stellar must not import console' >&2
	fail=1
fi

# Shared packages may expose public callbacks, but Stellar owns the public URL
# map. Keep route binding out of the shared handlers listed here.
for public_handler in \
	internal/comments/handler.go \
	internal/identity/public.go \
	internal/store/rating_handler.go \
	internal/store/vote_handler.go \
	internal/github/handler.go \
	internal/friends/friends.go; do
	if rg -n 'r\.(Group|GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\(' "$public_handler"; then
		printf 'module-check: shared public handler owns routes: %s\n' "$public_handler" >&2
		fail=1
	fi
done

# The repository is a single Go module. These paths were retired by the
# modular layout and must not quietly return through a new worktree or copy.
for legacy in backend cmd/server go.work go.work.sum; do
	if [ -e "$legacy" ]; then
		printf 'module-check: retired path exists: %s\n' "$legacy" >&2
		fail=1
	fi
done

# Validation artifacts may contain a copied module cache (for example
# .tmp/gomodcache). They are intentionally outside the repository module.
go_mod_count=$(find . \( -type d -name .tmp -o -type d -path './resource' \) -prune -o -name go.mod -type f -not -path './.git/*' -print | wc -l | tr -d ' ')
if [ "$go_mod_count" -ne 1 ]; then
	printf 'module-check: expected exactly one go.mod, found %s\n' "$go_mod_count" >&2
	fail=1
fi

# cmd/blog is a composition root. It may depend on the top-level module
# packages only; subpackages remain implementation details of those modules.
if rg -n '"blog/modules/(console|stellar)/' cmd/blog --glob '*.go'; then
	printf '%s\n' 'module-check: cmd/blog may import only blog/modules/console and blog/modules/stellar' >&2
	fail=1
fi

if [ "$fail" -ne 0 ]; then
	exit 1
fi
printf '%s\n' 'module-check: dependency direction passed'
