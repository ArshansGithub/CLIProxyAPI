#!/usr/bin/env bash
# Fetch the upstream model catalogs, show the diff against the embedded
# snapshot, and only overwrite after an explicit yes.
set -euo pipefail
BASE=https://raw.githubusercontent.com/router-for-me/models/refs/heads/main
DIR=internal/registry/models
TMP=$(mktemp -d)
for f in models.json codex_client_models.json; do
  curl -fsSL "$BASE/$f" -o "$TMP/$f"
  python3 -m json.tool "$TMP/$f" >/dev/null || { echo "$f is not valid JSON; aborting"; exit 1; }
  echo "=== diff $f (embedded → remote) ==="
  diff -u "$DIR/$f" "$TMP/$f" || true
done
read -r -p "Apply these catalogs to the embedded snapshot? [y/N] " ans
[ "$ans" = "y" ] || { echo "aborted"; exit 1; }
cp "$TMP"/models.json "$TMP"/codex_client_models.json "$DIR/"
M=$(shasum -a 256 "$DIR/models.json" | cut -d' ' -f1)
C=$(shasum -a 256 "$DIR/codex_client_models.json" | cut -d' ' -f1)
{
  echo "source=$BASE/"
  echo "models.json.sha256=$M"
  echo "codex_client_models.json.sha256=$C"
  echo "snapshot-from=refresh-models $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "reviewed=$(date -u +%Y-%m-%d)"
} > "$DIR/MODELS_VERSION"
echo "updated; review with: git diff $DIR"
