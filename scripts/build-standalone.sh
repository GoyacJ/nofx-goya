#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "==> Installing frontend dependencies"
npm --prefix web ci

echo "==> Building frontend (web/dist)"
npm --prefix web run build

echo "==> Building NOFX standalone binary"
go build -o nofx .

echo "✅ Build complete: ${ROOT_DIR}/nofx"
