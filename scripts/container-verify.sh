#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

CPUS="${CCX_CONTAINER_CPUS:-4}"
MEMORY="${CCX_CONTAINER_MEMORY:-8G}"
GO_IMAGE="${CCX_CONTAINER_GO_IMAGE:-golang:1.25-alpine}"
BUN_IMAGE="${CCX_CONTAINER_BUN_IMAGE:-oven/bun:alpine}"
GO_PROXY="${CCX_CONTAINER_GOPROXY:-https://goproxy.cn,direct}"
ALPINE_MIRROR="${CCX_CONTAINER_ALPINE_MIRROR:-mirrors.aliyun.com}"

if ! command -v container >/dev/null 2>&1; then
  echo "Apple Container CLI is not installed." >&2
  exit 1
fi

if ! container system status >/dev/null 2>&1; then
  echo "Apple Container is not running. Run: container system start" >&2
  exit 1
fi

ensure_volume() {
  local name="$1"
  local size="$2"

  if container volume inspect "$name" >/dev/null 2>&1; then
    return
  fi
  container volume create -s "$size" "$name" >/dev/null
}

stage_command='sh /workspace/scripts/container-stage-source.sh /workspace /work'

run_go() {
  ensure_volume ccx-go-mod 4G
  ensure_volume ccx-go-build 4G

  echo "==> Verifying Go backend in Linux/arm64"
  container run --rm --progress plain \
    --cpus "$CPUS" \
    --memory "$MEMORY" \
    --env "GOPROXY=$GO_PROXY" \
    --mount "type=bind,source=$ROOT_DIR,target=/workspace,readonly" \
    --mount type=volume,source=ccx-go-mod,target=/go/pkg/mod \
    --mount type=volume,source=ccx-go-build,target=/root/.cache/go-build \
    --workdir / \
    "$GO_IMAGE" sh -c "set -eu; $stage_command; cd /work/backend-go; export PATH=/usr/local/go/bin:\$PATH; go version; go test ./... -count=1; go vet ./...; CGO_ENABLED=0 go build -buildvcs=false -o /tmp/ccx-go ."
}

run_frontend() {
  ensure_volume ccx-bun-cache 2G
  ensure_volume ccx-bun-modules 4G

  echo "==> Verifying frontend in Linux/arm64"
  container run --rm --progress plain \
    --cpus "$CPUS" \
    --memory "$MEMORY" \
    --user root \
    --env "CCX_ALPINE_MIRROR=$ALPINE_MIRROR" \
    --mount "type=bind,source=$ROOT_DIR,target=/workspace,readonly" \
    --mount type=volume,source=ccx-bun-cache,target=/root/.bun/install/cache \
    --mount type=volume,source=ccx-bun-modules,target=/work/frontend/node_modules \
    --workdir / \
    "$BUN_IMAGE" sh -c "set -eu; $stage_command; cd /work/frontend; sed -i \"s#dl-cdn.alpinelinux.org#\$CCX_ALPINE_MIRROR#g\" /etc/apk/repositories; apk add --no-cache nodejs libstdc++ libgcc >/dev/null; bun install --frozen-lockfile; ./node_modules/.bin/vue-tsc --noEmit; ./node_modules/.bin/vite build --outDir /tmp/ccx-frontend-dist"
}

case "${1:-all}" in
  all)
    run_go
    run_frontend
    ;;
  go)
    run_go
    ;;
  frontend)
    run_frontend
    ;;
  *)
    echo "Usage: $0 [all|go|frontend]" >&2
    exit 2
    ;;
esac

echo "Apple Container verification passed."
