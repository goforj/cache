#!/usr/bin/env bash

set -euo pipefail

if ! command -v wgo >/dev/null 2>&1; then
  echo "Installing wgo..."
  go install github.com/bokwoon95/wgo@latest
fi

echo "Watching for .go file changes to regenerate documentation..."

echo "Starting API/examples watcher (non-test .go files)..."
wgo -verbose -file=.go -xfile '_test\.go$' -xdir examples \
  go run ./docs/examplegen/main.go :: \
  go run ./docs/readme/main.go &

echo "Starting test badge watcher (_test.go files, runs tests)..."
wgo -verbose -file '_test\.go$' \
  go run ./docs/readme/testcounts/main.go &

wait
