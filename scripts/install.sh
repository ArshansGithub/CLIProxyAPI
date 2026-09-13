#!/usr/bin/env bash
# Install dist/cliproxyapi over the Homebrew binary, restart the service, check health.
set -euo pipefail
BIN=/opt/homebrew/opt/cliproxyapi/bin/cliproxyapi
SRC="${1:-dist/cliproxyapi}"
[ -x "$SRC" ] || { echo "missing $SRC (run make build)"; exit 1; }
# First install has no binary to save. Keep going, but say so: `make rollback`
# needs $BIN.prev and there will not be one.
if [ -e "$BIN" ]; then
  cp "$BIN" "$BIN.prev"
else
  echo "no previous binary; rollback unavailable"
fi
cp "$SRC" "$BIN.new" && mv "$BIN.new" "$BIN"
brew services restart cliproxyapi >/dev/null
for _ in $(seq 1 30); do
  if curl -fsS -m 2 http://127.0.0.1:8317/healthz >/dev/null 2>&1; then
    # A healthy proxy is not necessarily *this* proxy: brew could have restarted
    # an older binary, or an upstream build could have been installed by hand.
    # `-v` prints the version and then exits 2, so read it before trusting it.
    VERSION=$("$BIN" -v 2>&1 | head -1 || true)
    case "$VERSION" in
      *-locked*)
        echo "healthy; running: $VERSION"; exit 0 ;;
      *)
        echo "installed binary reports \"$VERSION\", which is not a -locked build."
        echo "The running proxy is not the privacy-locked fork; run: make rollback"
        exit 1 ;;
    esac
  fi
  sleep 1
done
echo "proxy did not become healthy in 30s; run: make rollback"; exit 1
