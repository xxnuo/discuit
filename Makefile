SHELL := /bin/bash

UI_DIR := ui
UI_DEPS_STAMP := $(UI_DIR)/node_modules/.installed
BINARY := discuit
CONFIG_FILE := config.yaml
CONFIG_DEFAULT_FILE := config.default.yaml
PORTS_FILE := .dev-ports.mk
UI_CONFIG_FILE := $(UI_DIR)/pnpm-workspace.yaml
UI_INSTALL := pnpm install --frozen-lockfile
UI_RUN := pnpm
UI_LOCKFILE := $(UI_DIR)/pnpm-lock.yaml
BACKEND_PORT ?=
FRONTEND_PORT ?=
BACKEND_PORT_SEED := 38180
FRONTEND_PORT_SEED := 45173

.PHONY: dev dev-ui dev-server build build-ui build-server ensure-config ensure-ports migrate prepare-dev

dev: ensure-config ensure-ports migrate $(UI_DEPS_STAMP)
	backend_port='$(BACKEND_PORT)'; \
	frontend_port='$(FRONTEND_PORT)'; \
	if [ -z "$$backend_port" ] || [ -z "$$frontend_port" ]; then \
		. $(PORTS_FILE); \
		if [ -z "$$backend_port" ]; then backend_port=$$BACKEND_PORT; fi; \
		if [ -z "$$frontend_port" ]; then frontend_port=$$FRONTEND_PORT; fi; \
	fi; \
	trap 'kill 0' EXIT INT TERM; \
	DISCUIT_ADDR=:$$backend_port go run . serve & \
	(cd $(UI_DIR) && DISCUIT_ADDR=:$$backend_port VITE_PORT=$$frontend_port VITE_DEV_PROXY=http://127.0.0.1:$$backend_port $(UI_RUN) dev) & \
	wait

dev-ui: ensure-config ensure-ports $(UI_DEPS_STAMP)
	backend_port='$(BACKEND_PORT)'; \
	frontend_port='$(FRONTEND_PORT)'; \
	if [ -z "$$backend_port" ] || [ -z "$$frontend_port" ]; then \
		. $(PORTS_FILE); \
		if [ -z "$$backend_port" ]; then backend_port=$$BACKEND_PORT; fi; \
		if [ -z "$$frontend_port" ]; then frontend_port=$$FRONTEND_PORT; fi; \
	fi; \
	cd $(UI_DIR) && DISCUIT_ADDR=:$$backend_port VITE_PORT=$$frontend_port VITE_DEV_PROXY=http://127.0.0.1:$$backend_port $(UI_RUN) dev

dev-server: ensure-config ensure-ports migrate
	backend_port='$(BACKEND_PORT)'; \
	if [ -z "$$backend_port" ]; then \
		. $(PORTS_FILE); \
		backend_port=$$BACKEND_PORT; \
	fi; \
	DISCUIT_ADDR=:$$backend_port go run . serve

build: build-server build-ui

build-server:
	go build -o $(BINARY) .

build-ui: ensure-config $(UI_DEPS_STAMP)
	cd $(UI_DIR) && $(UI_RUN) build

ensure-config:
	@if [ ! -f $(CONFIG_FILE) ]; then cp $(CONFIG_DEFAULT_FILE) $(CONFIG_FILE); fi

ensure-ports:
	@if [ ! -f $(PORTS_FILE) ]; then \
		backend=$(BACKEND_PORT_SEED); \
		while lsof -nP -iTCP:$$backend -sTCP:LISTEN >/dev/null 2>&1; do backend=$$((backend + 1)); done; \
		frontend=$(FRONTEND_PORT_SEED); \
		while [ "$$frontend" = "$$backend" ] || lsof -nP -iTCP:$$frontend -sTCP:LISTEN >/dev/null 2>&1; do frontend=$$((frontend + 1)); done; \
		printf 'BACKEND_PORT=%s\nFRONTEND_PORT=%s\n' "$$backend" "$$frontend" > $(PORTS_FILE); \
	fi

migrate: ensure-config
	go run . migrate run

$(UI_DEPS_STAMP): $(UI_DIR)/package.json $(UI_LOCKFILE) $(UI_CONFIG_FILE)
	cd $(UI_DIR) && $(UI_INSTALL)
	touch $(UI_DEPS_STAMP)

prepare-dev:
	@if docker ps --format '{{.Names}}' | grep -q '^discuit-redis$$'; then \
		echo "Redis container already running."; \
	elif docker ps -a --format '{{.Names}}' | grep -q '^discuit-redis$$'; then \
		echo "Starting existing Redis container..."; \
		docker start discuit-redis; \
	else \
		echo "Creating Redis container..."; \
		docker run -d --name discuit-redis -p 6379:6379 redis:alpine; \
	fi
