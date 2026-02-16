#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "==> Installing frontend dependencies"
npm --prefix web ci

echo "==> Building frontend (web/dist)"
npm --prefix web run build

echo "==> Starting NOFX (single process on :8080 by default)"
go run . "$@"
