#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
exec sh "$SCRIPT_DIR/smoke_start_23_11_3h.sh" "${1:-http://127.0.0.1:8080}"
