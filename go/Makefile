# ============================================================================
# twinstar-bosskills - Go + ClickHouse build/dev pipeline
#
# Values come from .env if present; override on the command line like
#   make sync ARGS="--realm Helios --from-date 2026-06-20"
#   BK_HTTP_ADDR=:8080 make server
# ============================================================================

# ---- .env autoload ----------------------------------------------------------
# GNU make doesn't read .env on its own - pull in .env.example first (the
# committed defaults) and then .env to override. This way `make sync` works
# out of the box on a fresh checkout, and users only need to put secrets in
# .env. Both files are optional.
ENV_FILE         ?= .env
ENV_DEFAULT_FILE ?= .env.example
ifneq (,$(wildcard $(ENV_DEFAULT_FILE)))
include $(ENV_DEFAULT_FILE)
endif
ifneq (,$(wildcard $(ENV_FILE)))
include $(ENV_FILE)
endif
export

# ---- variables --------------------------------------------------------------
# Tooling
GO         ?= go
TEMPL      ?= templ
WGO        ?= wgo
TAILWIND   ?= ./bin/tailwindcss

# Paths
BIN_DIR    := bin
STATIC_DIR := internal/web/static
INPUT_CSS  := input.css
OUTPUT_CSS := $(STATIC_DIR)/app.css

# Commands (one binary per ./cmd/<name>)
COMMANDS := server sync migrate rename-repair ch-health
BINS     := $(addprefix $(BIN_DIR)/,$(COMMANDS))

# Build flags
GIT_SHA     := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(GIT_SHA)
BUILD_FLAGS := -trimpath -ldflags="$(LDFLAGS)"

# Tailwind binary download (Linux x64; override for other platforms).
TAILWIND_URL ?= https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64

# CLI arg passthrough - `make sync ARGS="--realm Helios"`
ARGS ?=

.DEFAULT_GOAL := help

.PHONY: help setup tidy gen gen-templ gen-css build clean \
        dev server sync migrate rename-repair ch-health \
        test fmt vet check \
        docker-up docker-down env

# ---- help (default) ---------------------------------------------------------
help:
	@echo "twinstar-bosskills - common targets"
	@echo
	@echo "  make setup        install templ/wgo, fetch tailwindcss binary"
	@echo "  make dev          run templ + tailwind watchers + wgo-reloaded server"
	@echo "  make build        produce $(BINS)"
	@echo "  make server       run server in foreground"
	@echo "  make migrate ARGS=up"
	@echo "  make sync ARGS=\"--realm Helios --from-date YYYY-MM-DD\""
	@echo "  make rename-repair ARGS=\"--realm Helios --dry-run\""
	@echo "  make ch-health    ClickHouse diagnostics (ARGS=\"--list\" for sections)"
	@echo "  make check        gen-templ + vet + tests"
	@echo "  make docker-up    bring up ClickHouse via docker compose"
	@echo "  make env          print loaded .env values"

# ---- setup ------------------------------------------------------------------
setup: tidy
	$(GO) install github.com/a-h/templ/cmd/templ@latest
	$(GO) install github.com/bokwoon95/wgo@latest
	@mkdir -p $(BIN_DIR)
	@test -x $(TAILWIND) || ( \
		echo "fetching tailwindcss standalone..." && \
		curl -sSL -o $(TAILWIND) $(TAILWIND_URL) && \
		chmod +x $(TAILWIND) \
	)
	@test -f $(ENV_FILE) || ( \
		echo "creating $(ENV_FILE) from $(ENV_DEFAULT_FILE)" && \
		cp $(ENV_DEFAULT_FILE) $(ENV_FILE) \
	)

tidy:
	$(GO) mod download

env:
	@echo "ENV_FILE              = $(ENV_FILE)"
	@echo "BK_CH_DSN             = $(BK_CH_DSN)"
	@echo "BK_TWINSTAR_API_URL   = $(BK_TWINSTAR_API_URL)"
	@echo "BK_HTTP_ADDR          = $(BK_HTTP_ADDR)"
	@echo "BK_ICON_DIR           = $(BK_ICON_DIR)"
	@echo "BK_ITEM_DIR           = $(BK_ITEM_DIR)"
	@echo "SECRET_TOKEN_GUILD    = $(if $(SECRET_TOKEN_GUILD),(set),(unset))"
	@echo "SECRET_TOKEN_ADMIN    = $(if $(SECRET_TOKEN_ADMIN),(set),(unset))"

# ---- generators -------------------------------------------------------------
gen: gen-templ gen-css

gen-templ:
	$(TEMPL) generate

gen-css: $(TAILWIND)
	$(TAILWIND) -i $(INPUT_CSS) -o $(OUTPUT_CSS) --minify

# Auto-fetch tailwindcss if missing.
$(TAILWIND):
	@$(MAKE) setup

# ---- build / run ------------------------------------------------------------
build: gen
	@mkdir -p $(BIN_DIR)
	@for cmd in $(COMMANDS); do \
		echo "building $$cmd"; \
		$(GO) build $(BUILD_FLAGS) -o $(BIN_DIR)/$$cmd ./cmd/$$cmd; \
	done

server: gen
	$(GO) run ./cmd/server

sync:
	$(GO) run ./cmd/sync $(ARGS)

migrate:
	$(GO) run ./cmd/migrate $(ARGS)

rename-repair:
	$(GO) run ./cmd/rename-repair $(ARGS)

ch-health:
	@$(GO) run ./cmd/ch-health $(ARGS)

# ---- dev --------------------------------------------------------------------
# Three parallel watchers: templ generate, tailwind --watch, wgo-restarted
# server. wgo only restarts on hand-written .go changes (not _templ.go).
dev: gen
	@echo "Starting dev: templ + tailwind + wgo. Ctrl-C to stop."
	@set -eu; \
	kill_tree() { \
		pid="$$1"; \
		if command -v pgrep >/dev/null 2>&1; then \
			for child in $$(pgrep -P "$$pid" 2>/dev/null || true); do \
				kill_tree "$$child"; \
			done; \
		fi; \
		kill "$$pid" 2>/dev/null || true; \
	}; \
	cleanup() { \
		status=$$?; \
		trap - INT TERM EXIT; \
		echo "Stopping dev processes..."; \
		for pid in $${templ_pid:-} $${tailwind_pid:-} $${wgo_pid:-}; do \
			if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
				kill_tree "$$pid"; \
			fi; \
		done; \
		wait 2>/dev/null || true; \
		exit "$$status"; \
	}; \
	trap cleanup INT TERM EXIT; \
	$(TEMPL) generate --watch & templ_pid=$$!; \
	$(TAILWIND) -i $(INPUT_CSS) -o $(OUTPUT_CSS) --watch & tailwind_pid=$$!; \
	$(WGO) -file=.go -xfile=_templ.go $(GO) run ./cmd/server & wgo_pid=$$!; \
	wait

# ---- quality ----------------------------------------------------------------
test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

check: gen-templ vet test

# ---- docker -----------------------------------------------------------------
docker-up:
	docker compose up -d

docker-down:
	docker compose down

# ---- clean ------------------------------------------------------------------
clean:
	rm -rf $(BIN_DIR) $(OUTPUT_CSS)
	find internal/web/views -name '*_templ.go' -delete
