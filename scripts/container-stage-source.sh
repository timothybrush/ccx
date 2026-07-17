#!/bin/sh

set -eu

source_dir="${1:-/workspace}"
target_dir="${2:-/work}"

mkdir -p "$target_dir"

# Copy only build inputs, keeping runtime secrets and host artifacts out of the VM worktree.
tar -C "$source_dir" \
  --exclude='backend-go/.config' \
  --exclude='backend-go/.env' \
  --exclude='backend-go/.env.*' \
  --exclude='backend-go/logs' \
  --exclude='frontend/.env' \
  --exclude='frontend/.env.*' \
  --exclude='frontend/node_modules' \
  --exclude='frontend/dist' \
  -cf - Makefile VERSION backend-go frontend shared scripts \
  | tar -C "$target_dir" -xf -

# The backend package embeds frontend/dist at compile time. The real frontend is
# verified separately, so keep host artifacts excluded and satisfy go:embed only
# inside the disposable Linux worktree.
embedded_dist="$target_dir/backend-go/frontend/dist"
if [ ! -f "$embedded_dist/index.html" ]; then
  mkdir -p "$embedded_dist"
  printf '%s\n' '<!doctype html><title>CCX container verification</title>' \
    >"$embedded_dist/index.html"
fi
