#!/usr/bin/env bash
# Audit an upstream tag before adopting it. Writes docs/fork/audits/<TAG>.md.
# Read-only: fetches tags and GitHub metadata, never pushes, never checks out
# the working tree. The only paths written are docs/fork/audits/<TAG>.md and a
# temporary worktree under $TMPDIR that is removed on exit.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
TAG="${1:?usage: upstream-audit.sh vX.Y.Z}"
BASE=$(git describe --tags --abbrev=0 --match 'v[0-9]*' --exclude '*-locked.*')
OUT="docs/fork/audits/$TAG.md"
git rev-parse -q --verify "refs/tags/$TAG" >/dev/null || { echo "tag $TAG not found; run make upstream-fetch"; exit 1; }

GOVULNCHECK=$(command -v govulncheck || echo "$(go env GOPATH)/bin/govulncheck")

# clip N — pass stdin through, but stop after N lines and say how many were
# dropped. A bare `head -N` in a report silently hides findings, which is the
# one thing an audit must not do: a reader cannot tell a short section from a
# truncated one. Every cap in this script goes through here.
clip() {
  local max="$1" tmp total
  tmp=$(mktemp)
  cat > "$tmp"
  total=$(wc -l < "$tmp" | tr -d ' ')
  head -n "$max" "$tmp"
  if [ "$total" -gt "$max" ]; then
    echo "… ($((total - max)) more lines truncated; run the command directly for the rest)"
  fi
  rm -f "$tmp"
}

TMPROOT=$(mktemp -d)
WT="$TMPROOT/wt"
trap 'git worktree remove --force "$WT" >/dev/null 2>&1 || true; rm -rf "$TMPROOT"' EXIT
git worktree add -q --detach "$WT" "$TAG"

# Top-5 upstream contributors. `git log %an` records a display name, not a
# GitHub login, so collect both the login and the profile name and also allow a
# login that appears in the author email (the noreply.github.com form).
gh api 'repos/router-for-me/CLIProxyAPI/contributors?per_page=5' --jq '.[].login' > "$TMPROOT/logins" 2>/dev/null || : > "$TMPROOT/logins"
: > "$TMPROOT/identities"
while read -r login; do
  [ -n "$login" ] || continue
  printf '%s\n' "$login" >> "$TMPROOT/identities"
  gh api "users/$login" --jq '.name // empty' >> "$TMPROOT/identities" 2>/dev/null || true
done < "$TMPROOT/logins"
LOGIN_RE=$(paste -sd'|' "$TMPROOT/logins" | sed 's/|$//')

# Section 7 uses the same pattern and the same walk as
# internal/egress/coverage_test.go (rawTransport), against the tree at $TAG.
cat > "$TMPROOT/sites.py" <<'PY'
import os, re, sys
root = sys.argv[1]
rawTransport = re.compile(r'&?http\.Transport\{|&?websocket\.Dialer\{|\.Proxy\s*=[^=]|http\.Client\{[^}]*Transport:|new\(http\.Transport\)|proxy\.SOCKS5\(|proxy\.FromURL\(|net\.Dial(Timeout)?\(|net\.Dialer\{|tls\.Dial\(|http2\.Transport\{|redis\.Options\{|redis\.NewDialer|minio\.Options\{|sql\.Open\(|git\.PlainClone\(')
hits = []
for rel in ('internal', 'sdk', 'cmd'):
    for dirpath, _, names in os.walk(os.path.join(root, rel)):
        for n in names:
            if not n.endswith('.go') or n.endswith('_test.go'):
                continue
            p = os.path.join(dirpath, n)
            with open(p, 'rb') as fh:
                if rawTransport.search(fh.read().decode('utf-8', 'replace')):
                    hits.append(os.path.relpath(p, root))
print('\n'.join(sorted(hits)))
PY

{
echo "# Upstream audit: $BASE → $TAG"
echo; echo "Generated $(date -u +%Y-%m-%dT%H:%M:%SZ). Base=\`$BASE\` Target=\`$TAG\`."
echo
echo "\`make rebase TAG=$TAG\` refuses to run while this file has an unticked checklist"
echo "item, with one exemption: items starting with \`After install:\` can only be ticked"
echo "after the rebuilt binary is installed, so the gate ignores them. Tick those by hand"
echo "once the post-install checks in Task 14 have run. The gate exists to make a skipped"
echo "review visible, not to block the rebase forever."
echo; echo "## 1. Range"
echo "Commits: $(git rev-list --count "$BASE".."$TAG")"; echo
git log --format='%an' "$BASE".."$TAG" | sort | uniq -c | sort -rn | sed 's/^/    /'
echo; echo "## 2. Dependencies"; echo '```diff'
{ git diff "$BASE" "$TAG" -- go.mod go.sum | clip 200; } || true; echo '```'
echo; echo "govulncheck:"; echo '```'
if [ -x "$GOVULNCHECK" ]; then
  # Full output, not a tail: the summary AND every module-level finding must be
  # in the report. govulncheck output is bounded by the number of findings.
  (cd "$WT" && ("$GOVULNCHECK" ./... 2>&1 || true))
else
  echo "govulncheck not found; go install golang.org/x/vuln/cmd/govulncheck@latest"
fi
echo '```'
echo; echo "## 3. Commits (GPG status; [review] = author outside the top-5 upstream contributors)"; echo
git log --format='%h%x09%G?%x09%an%x09%ae%x09%s' "$BASE".."$TAG" | while IFS=$'\t' read -r h g a e rest; do
  mark=" [review]"
  if grep -Fxq "$a" "$TMPROOT/identities"; then
    mark=""
  elif [ -n "$LOGIN_RE" ] && [[ "$e" =~ ($LOGIN_RE) ]]; then
    mark=""
  fi
  echo "- \`$h\` $g $a$mark — $rest"
done
echo; echo "## 4. Added-line grep (non-test Go)"; echo '```'
git diff "$BASE" "$TAG" -- '*.go' ':!*_test.go' | grep -nE '^\+' | grep -E 'os/exec|syscall\.|dlopen|os\.WriteFile|os\.Create\(|func init\(\)|go:embed|base64\.|"(https|wss)://' | clip 120 || true
echo '```'
echo; echo "## 5. Watched paths (full diff)"
for p in internal/managementasset internal/pluginstore internal/pluginhost internal/registry internal/home; do
  echo; echo "### $p"; echo '```diff'; { git diff "$BASE" "$TAG" -- "$p" | clip 400; } || true; echo '```'
done
echo; echo '### files containing $TOKEN$'; echo '```'
(cd "$WT" && grep -rln '\$TOKEN\$' --include='*.go' internal sdk 2>/dev/null || true); echo '```'
echo; echo "## 6. Pins"
echo "- panel pin: $(grep '^tag=' internal/managementasset/panel/PANEL_VERSION)"
echo "- panel latest: $(gh release view --repo router-for-me/Cli-Proxy-API-Management-Center --json tagName --jq .tagName 2>/dev/null || echo unknown)"
echo "- models snapshot: $(grep '^reviewed=' internal/registry/models/MODELS_VERSION)"
echo; echo "## 7. New raw transport/dialer sites at $TAG"; echo '```'
python3 "$TMPROOT/sites.py" "$WT" | sed '/^$/d' | sort > "$TMPROOT/.sites"
grep -v '^#' internal/egress/coverage_allowlist.txt | sed '/^[[:space:]]*$/d' | sort > "$TMPROOT/.allowed"
comm -23 "$TMPROOT/.sites" "$TMPROOT/.allowed" || true; echo '```'
echo; echo "## 8. Checklist"
echo "- [ ] Read every [review] commit and every watched-path diff"
echo "- [ ] Dependency changes accepted (or none)"
echo "- [ ] Panel pin decision recorded (keep / bump via make panel-bump)"
echo "- [ ] Models snapshot decision recorded (keep / make refresh-models)"
echo "- [ ] New raw transport sites guarded and allowlisted (or none)"
echo "- [ ] After install: one Claude Code and one Codex session in egress audit mode showed no unexpected hosts"
echo; echo "## Notes"
echo
echo "One line per [review] commit and per watched-path change: what it does, whether it touches credentials/egress/panel/plugin/registry, accept/reject. Pin decisions and govulncheck follow-ups go here too."
} > "$OUT"
echo "wrote $OUT"
