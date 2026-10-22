#!/usr/bin/env bash
# build   : build the pinned panel tag from source into internal/managementasset/panel
# verify  : build the pinned tag to a temp dir and compare with the GitHub release asset
# bump T  : show the source diff pinned→T, ask, then build T and update the pin
set -euo pipefail
REPO=router-for-me/Cli-Proxy-API-Management-Center
DIR=internal/managementasset/panel
PIN="$DIR/PANEL_VERSION"

# Temp dirs are tracked in these globals (not `local`, and never populated via a
# command-substituted helper) so the EXIT trap can always see and remove them,
# even if a build/clone step aborts the script partway through via `set -e`.
BUILD_SRC=""
DIFF_SRC=""
TMP=""
cleanup() {
  set +e
  [ -n "${BUILD_SRC:-}" ] && rm -rf "$BUILD_SRC"
  [ -n "${DIFF_SRC:-}" ] && rm -rf "$DIFF_SRC"
  [ -n "${TMP:-}" ] && rm -rf "$TMP"
  return 0
}
trap cleanup EXIT

pinned_tag() { grep '^tag=' "$PIN" | cut -d= -f2; }
build_tag() { # $1 tag, $2 out-dir
  local tag="$1" out="$2"
  BUILD_SRC=$(mktemp -d)
  git clone -q --depth 1 --branch "$tag" "https://github.com/$REPO.git" "$BUILD_SRC"
  (cd "$BUILD_SRC" && bun install --frozen-lockfile >/dev/null && VERSION="$tag" bun run build >/dev/null)
  cp "$BUILD_SRC/dist/index.html" "$out/management.html"
  (cd "$BUILD_SRC" && git rev-parse HEAD) > "$out/.commit"
}
write_pin() { # $1 tag, $2 commit, $3 source
  local sum; sum=$(shasum -a 256 "$DIR/management.html" | cut -d' ' -f1)
  printf 'repo=%s\ntag=%s\ncommit=%s\nsha256=%s\nsource=%s\nreviewed=%s\n' "$REPO" "$1" "$2" "$sum" "$3" "$(date -u +%Y-%m-%d)" > "$PIN"
}
case "${1:-}" in
  build)
    tag=$(pinned_tag); TMP=$(mktemp -d); build_tag "$tag" "$TMP"
    cp "$TMP/management.html" "$DIR/management.html"; write_pin "$tag" "$(cat "$TMP/.commit")" "local-build"
    echo "built $tag → $DIR"; grep sha256 "$PIN" ;;
  verify)
    tag=$(pinned_tag); TMP=$(mktemp -d); build_tag "$tag" "$TMP"
    local_sum=$(shasum -a 256 "$TMP/management.html" | cut -d' ' -f1)
    gh release download "$tag" --repo "$REPO" --pattern management.html --dir "$TMP/rel" >/dev/null
    rel_sum=$(shasum -a 256 "$TMP/rel/management.html" | cut -d' ' -f1)
    echo "local build : $local_sum"; echo "release     : $rel_sum"
    [ "$local_sum" = "$rel_sum" ] && echo "PASS: release matches source build" || echo "MISMATCH: investigate before trusting release assets" ;;
  bump)
    new="${2:?usage: panel.sh bump vX.Y.Z}"; old=$(pinned_tag); DIFF_SRC=$(mktemp -d)
    git clone -q "https://github.com/$REPO.git" "$DIFF_SRC"
    echo "=== commits $old..$new ==="; (cd "$DIFF_SRC" && git log --format='%h %an %s' "$old".."$new")
    echo; echo "=== files ==="; (cd "$DIFF_SRC" && git diff --stat "$old".."$new" | tail -40)
    echo; echo "=== risky primitives in added lines ==="
    (cd "$DIFF_SRC" && git diff "$old".."$new" -- 'src/*' | grep -E '^\+' | grep -nE 'eval\(|new Function|fetch\(|XMLHttpRequest|WebSocket|sendBeacon|postMessage|localStorage|document\.cookie|import\(|"https?://' || echo "(none)")
    read -r -p "Build $new and update the pin? [y/N] " ans; [ "$ans" = "y" ] || { echo aborted; exit 1; }
    TMP=$(mktemp -d); build_tag "$new" "$TMP"; cp "$TMP/management.html" "$DIR/management.html"; write_pin "$new" "$(cat "$TMP/.commit")" "local-build"
    echo "pinned $new"; grep sha256 "$PIN" ;;
  *) echo "usage: panel.sh build|verify|bump TAG"; exit 1 ;;
esac
