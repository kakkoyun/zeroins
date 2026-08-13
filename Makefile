SHELL := /bin/bash

GO_FILES := $(shell find cmd internal tools -name '*.go' -type f -print | sort)

.DEFAULT_GOAL := help

.PHONY: build test fmt lint crossbuild check check/helm check/integration check/examples check/skills install help

build:
	go build ./...

test:
	go test -race -count=1 ./...

fmt:
	gofmt -w $(GO_FILES)
	goimports -w $(GO_FILES)

lint:
	command -v goimports >/dev/null || { printf '%s\n' 'Install goimports: go install golang.org/x/tools/cmd/goimports@v0.31.0' >&2; exit 1; }
	command -v typos >/dev/null || { printf '%s\n' 'Install typos: cargo install typos-cli' >&2; exit 1; }
	if gofmt -l $(GO_FILES) | grep -q .; then \
		printf '%s\n' 'Run make fmt; gofmt reported changes.' >&2; \
		exit 1; \
	fi
	if goimports -l $(GO_FILES) | grep -q .; then \
		printf '%s\n' 'Run make fmt; goimports reported changes.' >&2; \
		exit 1; \
	fi
	go vet ./...
	typos README.md skills docs scripts .github
	@if rg -n 'tools/(cli|skills)|go-instr-pull' README.md skills cmd internal; then \
		printf '%s\n' 'Found stale talk-repository tooling paths.' >&2; \
		exit 1; \
	fi

crossbuild:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./...
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./...
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...

check: lint build test crossbuild check/examples check/skills

check/helm:
	./scripts/verify-helm-contracts.sh

check/examples:
	./scripts/check-examples-drift.sh

check/skills:
	go run ./tools/skillcheck

check/integration:
	./scripts/integration-linux.sh

install:
	go install ./cmd/...

help:
	@printf '%s\n' 'Targets:'
	@printf '%s\n' '  make build              Build every package'
	@printf '%s\n' '  make test               Run tests with the race detector'
	@printf '%s\n' '  make fmt                Format Go source with gofmt and goimports'
	@printf '%s\n' '  make lint               Check formatting, vet, typos, and stale paths'
	@printf '%s\n' '  make crossbuild         Build Linux, Darwin, and Windows targets'
	@printf '%s\n' '  make check              Run lint, build, tests, and cross-builds'
	@printf '%s\n' '  make check/helm         Render and validate pinned Helm chart contracts'
	@printf '%s\n' '  make check/examples     Assert examples fixtures match integration references'
	@printf '%s\n' '  make check/skills        Validate skill directories, frontmatter, and registration'
	@printf '%s\n' '  make check/integration  Run the live Linux release gate'
	@printf '%s\n' '  make install            Install all four commands locally'
