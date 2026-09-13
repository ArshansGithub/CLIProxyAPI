#!/usr/bin/env bash
# Install dist/cliproxyapi over the Homebrew binary, restart the service, check health.
set -euo pipefail
BIN=/opt/homebrew/opt/cliproxyapi/bin/cliproxyapi
SRC="${1:-dist/cliproxyapi}"
[ -x "$SRC" ] || { echo "missing $SRC (run make build)"; exit 1; }
cp "$BIN" "$BIN.prev"
cp "$SRC" "$BIN.new" && mv "$BIN.new" "$BIN"
brew services restart cliproxyapi >/dev/null
for _ in $(seq 1 30); do
  if curl -fsS -m 2 http://127.0.0.1:8317/healthz >/dev/null 2>&1; then
    echo "healthy; running: $("$BIN" -v 2>&1 | head -1)"; exit 0
  fi
  sleep 1
done
echo "proxy did not become healthy in 30s; run: make rollback"; exit 1
