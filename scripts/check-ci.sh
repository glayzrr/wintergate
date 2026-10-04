#!/usr/bin/env bash
set -euo pipefail

if (( $# > 1 )); then
  printf 'Usage: %s [coverage-profile-path]\n' "$0" >&2
  exit 2
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd -- "$script_dir/.."

required_coverage=80
go_toolchain=go1.26.0
if [[ -n "${1:-}" ]]; then
  coverage_file="$1"
else
  coverage_file="$(mktemp "${TMPDIR:-/tmp}/wintergate-coverage.XXXXXX")"
  trap 'rm -f -- "$coverage_file"' EXIT
fi

# 통합 테스트가 호출하는 프로젝트 패키지의 실행 경로도 커버리지에 포함합니다.
printf 'using %s\n' "$go_toolchain"
GOTOOLCHAIN="$go_toolchain" go test -count=1 -coverpkg=./... -covermode=atomic -coverprofile="$coverage_file" ./...

total="$(GOTOOLCHAIN="$go_toolchain" go tool cover -func="$coverage_file" | awk '/^total:/ {gsub("%", "", $3); print $3}')"
if [[ -z "$total" ]]; then
  printf 'Failed to resolve total coverage.\n' >&2
  exit 1
fi

awk -v total="$total" -v required="$required_coverage" 'BEGIN {
  if ((total + 0) < required) {
    printf("coverage %.1f%% is below required %.1f%%\n", total + 0, required);
    exit 1;
  }
  printf("coverage %.1f%% meets required %.1f%%\n", total + 0, required);
}'

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  printf 'total=%s\nrequired=%s\n' "$total" "$required_coverage" >> "$GITHUB_OUTPUT"
fi
