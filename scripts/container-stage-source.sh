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
