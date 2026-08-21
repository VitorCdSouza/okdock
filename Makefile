SHELL := /bin/bash
API   := api
WEB   := web
DIST  := $(API)/internal/webui/dist

.DEFAULT_GOAL := help

.PHONY: help
help: ## Lista os alvos
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: dev-api
dev-api: ## Sobe a API em :8080, liberando CORS para o ng serve
	cd $(API) && GAMEDOCK_ROOT=$${GAMEDOCK_ROOT:-$(PWD)/.data} \
	  GAMEDOCK_ALLOW_ORIGIN=http://localhost:4200 \
	  go run ./cmd/gamedock

.PHONY: dev-web
dev-web: ## Sobe o Angular em :4200, com /api indo para a API local
	cd $(WEB) && npm start

.PHONY: build
build: build-web build-api ## Compila o binário com o frontend embutido

.PHONY: build-web
build-web:
	cd $(WEB) && npm ci && npm run build
	rm -rf $(DIST)
	mkdir -p $(DIST)
	cp -r $(WEB)/dist/web/browser/. $(DIST)/

.PHONY: build-api
build-api:
	cd $(API) && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o ../gamedock ./cmd/gamedock
	@echo "binário em ./gamedock"

.PHONY: test
test: test-api test-web ## Roda todos os testes

.PHONY: test-api
test-api: ## Testes da API (não precisam de daemon docker)
	cd $(API) && go test ./...

.PHONY: test-web
test-web:
	cd $(WEB) && npm run test -- --watch=false --browsers=ChromeHeadless

.PHONY: lint
lint: ## vet + gofmt
	cd $(API) && go vet ./...
	@out=$$(cd $(API) && gofmt -l .); \
	  if [ -n "$$out" ]; then echo "gofmt pendente em:"; echo "$$out"; exit 1; fi

.PHONY: clean
clean: ## Remove artefatos de build
	rm -f gamedock
	rm -rf $(WEB)/dist $(WEB)/.angular
	git checkout -- $(DIST) 2>/dev/null || true
