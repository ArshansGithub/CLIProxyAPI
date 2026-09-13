SHELL := /bin/bash
# The fork tags its own releases <upstream>-locked.N at the tip of `locked`, and
# those match 'v[0-9]*' too. Without --exclude, git describe would resolve the
# fork's own release tag as the base and every rebase range would be empty.
DESCRIBE_BASE := git describe --tags --abbrev=0 --match 'v[0-9]*' --exclude '*-locked.*'
TAG_UPSTREAM ?= $(shell $(DESCRIBE_BASE) 2>/dev/null)
COMMIT       := $(shell git rev-parse --short HEAD)
VERSION      ?= $(TAG_UPSTREAM)-locked.dev
LDFLAGS      := -s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) \
                -X main.DefaultConfigPath=/opt/homebrew/etc/cliproxyapi.conf
GOFLAGS_LOCKED := -tags locked

.PHONY: build verify verify-tagless install rollback upstream-fetch refresh-models \
        patches upstream-audit rebase panel-build panel-verify panel-bump

build:
	@mkdir -p dist
	go build $(GOFLAGS_LOCKED) -ldflags "$(LDFLAGS)" -o dist/cliproxyapi ./cmd/server
	@echo "built dist/cliproxyapi ($(VERSION) $(COMMIT))"

verify:
	go build $(GOFLAGS_LOCKED) ./...
	go vet $(GOFLAGS_LOCKED) ./...
	go test $(GOFLAGS_LOCKED) ./...

verify-tagless:
	go build ./... && go vet ./... && go test ./...

install: build
	./scripts/install.sh dist/cliproxyapi

rollback:
	@BIN=/opt/homebrew/opt/cliproxyapi/bin/cliproxyapi; \
	[ -f $$BIN.prev ] || { echo "no .prev binary"; exit 1; }; \
	cp $$BIN.prev $$BIN && brew services restart cliproxyapi && echo "rolled back"

upstream-fetch:
	git fetch upstream --tags --no-write-fetch-head 'refs/tags/*:refs/tags/*'
	@echo "latest upstream tag: $$(git tag --sort=-v:refname | head -1)"

refresh-models:
	./scripts/refresh-models.sh

patches:
	./scripts/patches.sh

upstream-audit:
	@[ -n "$(TAG)" ] || { echo "usage: make upstream-audit TAG=vX.Y.Z"; exit 1; }
	./scripts/upstream-audit.sh $(TAG)

# The audit checklist gates the rebase. Items whose text starts with
# "After install:" are exempt: they can only be ticked once the rebuilt binary
# is running, which is after this target. Every other item must be ticked.
rebase:
	@[ -n "$(TAG)" ] || { echo "usage: make rebase TAG=vX.Y.Z"; exit 1; }
	@[ -f docs/fork/audits/$(TAG).md ] || { echo "no audit report for $(TAG); run make upstream-audit TAG=$(TAG)"; exit 1; }
	@! grep '^- \[ \]' docs/fork/audits/$(TAG).md | grep -v '^- \[ \] After install:' \
	  || { echo "audit checklist for $(TAG) has unticked items (post-install items are exempt)"; exit 1; }
	git config rerere.enabled true
	@BASE=$$($(DESCRIBE_BASE)); \
	  N=$$(git rev-list --count $$BASE..locked); \
	  echo "base upstream tag: $$BASE ($$N fork commits to replay)"; \
	  [ "$$N" -gt 0 ] || { \
	    echo "refusing to rebase: BASE=$$BASE already contains every commit on locked,"; \
	    echo "so the replay range is empty and the rebase would move locked onto bare upstream."; \
	    echo "Check that BASE is an upstream tag and not one of the fork's own *-locked.* tags."; \
	    exit 1; }
	git branch -f locked-prev HEAD
	@BASE=$$($(DESCRIBE_BASE)); set -x; git rebase --onto $(TAG) $$BASE locked
	@N=$$(git log --format=%s $(TAG)..locked | grep -c '^lock:' || true); \
	  if [ "$$N" -gt 0 ]; then \
	    echo "rebased onto $(TAG); $$N lock: commits replayed"; \
	  else \
	    echo "*** WARNING: no lock: commits on locked after the rebase onto $(TAG)."; \
	    echo "*** The lockdown stack looks dropped. Recover with: git reset --hard locked-prev"; \
	  fi
	@echo "rebased onto $(TAG); now: make verify"

panel-build:
	./scripts/panel.sh build

panel-verify:
	./scripts/panel.sh verify

panel-bump:
	@[ -n "$(TAG)" ] || { echo "usage: make panel-bump TAG=vX.Y.Z"; exit 1; }
	./scripts/panel.sh bump $(TAG)
