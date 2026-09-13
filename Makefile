SHELL := /bin/bash
TAG_UPSTREAM ?= $(shell git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null)
COMMIT       := $(shell git rev-parse --short HEAD)
VERSION      ?= $(TAG_UPSTREAM)-locked.dev
LDFLAGS      := -s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) \
                -X main.DefaultConfigPath=/opt/homebrew/etc/cliproxyapi.conf
GOFLAGS_LOCKED := -tags locked

.PHONY: build verify verify-tagless install rollback upstream-fetch refresh-models \
        patches upstream-audit rebase

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
	git branch -f locked-prev HEAD
	git rebase --onto $(TAG) $$(git describe --tags --abbrev=0 --match 'v[0-9]*') locked
	@echo "rebased onto $(TAG); now: make verify"
