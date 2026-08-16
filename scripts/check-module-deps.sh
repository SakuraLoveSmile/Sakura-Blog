#!/bin/sh
set -eu

fail=0

if rg -n '"blog/modules/(console|stellar)' internal; then
	printf '%s\n' 'module-check: internal must not import business modules' >&2
	fail=1
fi
if rg -n '"blog/modules/stellar' modules/console; then
	printf '%s\n' 'module-check: console must not import stellar' >&2
	fail=1
fi
if rg -n '"blog/modules/console' modules/stellar; then
	printf '%s\n' 'module-check: stellar must not import console' >&2
	fail=1
fi

if [ "$fail" -ne 0 ]; then
	exit 1
fi
printf '%s\n' 'module-check: dependency direction passed'
