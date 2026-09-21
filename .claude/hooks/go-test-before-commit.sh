#!/usr/bin/env bash
# PreToolUse hook: run `go test ./...` before anything that creates a commit, and block it if
# the tests fail. Registered for Bash (only `git commit` commands) and for the GitHub MCP
# tools that commit to a repo (always).
input=$(cat)
tool=$(jq -r '.tool_name // empty' <<<"$input")

if [[ "$tool" == Bash ]]; then
  cmd=$(jq -r '.tool_input.command // empty' <<<"$input")
  # Match `git commit` anywhere in the command, including after cd/&&/;/| and with git
  # options like `git -C dir commit`.
  if ! grep -qE '(^|[;&|(`[:space:]])git([[:space:]]+-[^[:space:]]+([[:space:]]+[^-[:space:]][^[:space:]]*)?)*[[:space:]]+commit([[:space:]]|$)' <<<"$cmd"; then
    exit 0
  fi
fi

cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0
if ! out=$(go -C backend test ./... 2>&1); then
  echo "Commit blocked ($tool): go test ./... failed." >&2
  echo "$out" | tail -n 40 >&2
  exit 2
fi
exit 0
