#!/bin/bash
set -e

export CGO_ENABLED=0
export GOPROXY=https://goproxy.cn,direct

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

VERSION=$(cat VERSION 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS="-s -w -X main.Version=${VERSION} -extldflags '-static -fpic'"

mkdir -p dist

echo "=== Building buaa-login ==="
echo "Version: ${VERSION}"
echo "Build time: ${BUILD_TIME}"
echo ""

# x86_64 (linux/amd64)
echo "[1/4] Building linux/amd64 (x86_64)..."
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o "dist/buaa-login_linux_amd64" ./cmd/buaa-login/

# arm64 (linux/arm64)
echo "[2/4] Building linux/arm64 (arm64)..."
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$LDFLAGS" -o "dist/buaa-login_linux_arm64" ./cmd/buaa-login/

# armv8 (linux/arm64, same as arm64)
echo "[3/4] Building linux/arm64 (armv8)..."
cp dist/buaa-login_linux_arm64 dist/buaa-login_linux_armv8

# mipsel_24kc (linux/mipsle)
echo "[4/4] Building linux/mipsle (mipsel_24kc)..."
GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -trimpath -ldflags "$LDFLAGS" -o "dist/buaa-login_linux_mipsel_24kc" ./cmd/buaa-login/

echo ""
echo "=== Build complete ==="
ls -lh dist/
