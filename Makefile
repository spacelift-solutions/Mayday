GIT_HOOKS_REPOSITORY ?= https://github.com/spacelift-solutions/git-hooks.git
XDG_DATA_HOME ?= $(HOME)/.local/share
GIT_HOOKS_DIR ?= $(XDG_DATA_HOME)/spacelift-git-hooks

.PHONY: hooks

hooks:
	@if [ -d "$(GIT_HOOKS_DIR)/.git" ]; then \
		git -C "$(GIT_HOOKS_DIR)" pull --ff-only; \
	else \
		git clone "$(GIT_HOOKS_REPOSITORY)" "$(GIT_HOOKS_DIR)"; \
	fi
	@"$(GIT_HOOKS_DIR)/install"
