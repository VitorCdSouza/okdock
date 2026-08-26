SHELL := /bin/bash
API   := api
WEB   := web
DATA  ?= $(PWD)/.data
DIST  := $(API)/internal/webui/dist
IMAGE := ghcr.io/vitorcdsouza/okdock:latest
SERVER      ?= vitorcds@192.168.0.100
SERVER_DIR  ?= servidor/okdock

.DEFAULT_GOAL := help

.PHONY: help
help: ## List the targets
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: dev
dev: ## Bring API and web up in one terminal, Ctrl-C takes both down
	@echo "  API  http://localhost:8080"
	@echo "  web  http://localhost:4200   <- open this one"
	@echo "  data in $(DATA)"
	@echo "  (use localhost, not 127.0.0.1: ng serve listens on IPv6 only)"
	@echo
	@trap 'trap - INT TERM EXIT; kill 0' INT TERM EXIT; \
	  ( cd $(API) && OKDOCK_ROOT=$(DATA) OKDOCK_CONFIG=$(DATA)/.config OKDOCK_TEMPLATES=$(DATA)/templates OKDOCK_ALLOW_ORIGIN=http://localhost:4200 \
	      go run ./cmd/okdock 2>&1 | sed -u 's/^/[api] /' ) & \
	  ( cd $(WEB) && npm start 2>&1 | sed -u 's/^/[web] /' ) & \
	  wait

.PHONY: dev-api
dev-api: ## API only, on :8080
	cd $(API) && OKDOCK_ROOT=$(DATA) OKDOCK_CONFIG=$(DATA)/.config OKDOCK_TEMPLATES=$(DATA)/templates \
	  OKDOCK_ALLOW_ORIGIN=http://localhost:4200 \
	  go run ./cmd/okdock

.PHONY: dev-web
dev-web: ## Angular only, on :4200
	cd $(WEB) && npm start

.PHONY: build
build: build-web build-api ## Build the binary with the frontend embedded

.PHONY: build-web
build-web:
	cd $(WEB) && npm ci && npm run build
	rm -rf $(DIST)
	mkdir -p $(DIST)
	cp -r $(WEB)/dist/web/browser/. $(DIST)/

.PHONY: build-api
build-api:
	cd $(API) && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o ../okdock ./cmd/okdock
	@echo "binary at ./okdock"

.PHONY: test
test: test-api test-web ## Run every test

.PHONY: test-api
test-api: ## API tests, no docker daemon needed
	cd $(API) && go test ./...

.PHONY: test-web
test-web:
	cd $(WEB) && npm run test -- --watch=false --browsers=ChromeHeadless

.PHONY: lint
lint: ## vet + gofmt
	cd $(API) && go vet ./...
	@out=$$(cd $(API) && gofmt -l .); \
	  if [ -n "$$out" ]; then echo "gofmt pending in:"; echo "$$out"; exit 1; fi

.PHONY: deploy
deploy: ## build here, hand the image to the server, recreate the container
	docker build -t $(IMAGE) .
	docker save $(IMAGE) | ssh $(SERVER) 'docker load'
	scp -q docker-compose.yml $(SERVER):$(SERVER_DIR)/docker-compose.yml
	ssh $(SERVER) 'cd $(SERVER_DIR) && docker compose up -d --pull never'
	@ssh $(SERVER) 'curl -fs http://localhost:8090/api/v1/health' && echo

.PHONY: clean
clean: ## Remove build artifacts
	rm -f okdock
	rm -rf $(WEB)/dist $(WEB)/.angular
	git checkout -- $(DIST) 2>/dev/null || true
