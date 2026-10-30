GIT_HOOKS_REPOSITORY ?= https://github.com/spacelift-solutions/git-hooks.git
XDG_DATA_HOME ?= $(HOME)/.local/share
GIT_HOOKS_DIR ?= $(XDG_DATA_HOME)/spacelift-git-hooks
PYTHON ?= python3

.PHONY: hooks

.PHONY: dev build build-linux test smoke check flows-check manifests-check
dev:
	python3 scripts/local.py

build:
	go build -trimpath -o bin/mayday ./cmd/mayday

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/linux-amd64/mayday ./cmd/mayday

test:
	go test -race ./...
	go vet ./...

manifests-check:
	$(PYTHON) scripts/validate_manifests.py

smoke:
	python3 scripts/smoke.py

flows-check:
	PYTHON=$(PYTHON) node --test flows/test/*.test.mjs

check: test flows-check
	test -z "$$(gofmt -l cmd internal)"
	tofu fmt -check -recursive infra flows/deploy

hooks:
	@if [ -d "$(GIT_HOOKS_DIR)/.git" ]; then \
		git -C "$(GIT_HOOKS_DIR)" pull --ff-only; \
	else \
		git clone "$(GIT_HOOKS_REPOSITORY)" "$(GIT_HOOKS_DIR)"; \
	fi
	@"$(GIT_HOOKS_DIR)/install"
