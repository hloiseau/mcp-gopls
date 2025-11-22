#!/usr/bin/env bash

set -euo pipefail

tmpdir="$(mktemp -d)"

cleanup() {
	mv "${tmpdir}/go.mod" go.mod
	if [[ -f "${tmpdir}/go.sum" ]]; then
		mv "${tmpdir}/go.sum" go.sum
	else
		rm -f go.sum >/dev/null 2>&1 || true
	fi
	rm -rf "${tmpdir}"
}
trap cleanup EXIT

cp go.mod "${tmpdir}/go.mod"
if [[ -f go.sum ]]; then
	cp go.sum "${tmpdir}/go.sum"
fi

status=0

if ! go mod tidy; then
	echo "go mod tidy failed"
	exit 1
fi

if ! cmp -s go.mod "${tmpdir}/go.mod"; then
	echo "go.mod is not tidy; run 'go mod tidy'"
	status=1
fi

if [[ -f go.sum ]]; then
	if [[ -f "${tmpdir}/go.sum" ]]; then
		if ! cmp -s go.sum "${tmpdir}/go.sum"; then
			echo "go.sum is not tidy; run 'go mod tidy'"
			status=1
		fi
	else
		echo "go.sum was created by tidy; run 'go mod tidy' and commit the results"
		status=1
	fi
else
	if [[ -f "${tmpdir}/go.sum" ]]; then
		echo "go.sum was removed by tidy; run 'go mod tidy' and commit the results"
		status=1
	fi
fi

exit $status

