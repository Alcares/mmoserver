#!/usr/bin/env bash
# PostToolUse hook (Write|Edit): gofmt any .go file that was just touched.
f=$(jq -r '.tool_input.file_path // empty')
case "$f" in
  *.go) gofmt -w "$f" 2>/dev/null || true ;;
esac
