.PHONY: test cover-strict lint run build docker cover web web-test e2e screenshots docs audit ship

COVER_MIN ?= 85

test: ## set KYBER_TEST_DATABASE_URL to include Postgres integration tests
	go test -race -timeout 30m -coverpkg=./internal/...,./web/... -coverprofile=coverage.out ./...
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/ {sub("%","",$$3); print $$3}'); \
	echo "total coverage: $$total% (min $(COVER_MIN)%)"; \
	awk -v t=$$total -v m=$(COVER_MIN) 'BEGIN { exit (t+0 < m+0) }'

cover-strict: ## packages listed in .cover-strict must stay at 100% statement coverage
	@fail=0; for p in $$(grep -v '^#' .cover-strict); do \
	  c=$$(go test -cover $$p | grep -o '[0-9.]*% of statements' | cut -d% -f1); \
	  if [ "$$c" != "100.0" ]; then echo "FAIL $$p: $$c% (need 100%)"; fail=1; else echo "ok   $$p: 100%"; fi; \
	done; exit $$fail

lint:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo "gofmt needed" && exit 1)
	go vet ./...

web: ## build the React UI into web/dist (embedded by the Go build)
	npm --prefix web ci --no-audit --no-fund
	npm --prefix web run build

web-test:
	npm --prefix web run typecheck
	npm --prefix web test

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/kyber ./cmd/kyber

run:
	go run ./cmd/kyber

screenshots: web build ## regenerates the README screenshots in docs/assets from a seeded in-memory server
	@KYBER_ADDR=:18080 KYBER_COOKIE_SECURE=false KYBER_ADMIN_EMAILS=admin@e2e.test KYBER_ATTACHMENTS_DIR=$$(mktemp -d) ./bin/kyber > screenshots.log 2>&1 & pid=$$!; \
	sleep 1; KYBER_URL=http://localhost:18080 node web/scripts/screenshots.mjs; rc=$$?; kill $$pid; exit $$rc

e2e: build ## runs Playwright against a fresh in-memory server on :18080
	@KYBER_ADDR=:18080 KYBER_COOKIE_SECURE=false KYBER_ADMIN_EMAILS=admin@e2e.test KYBER_ATTACHMENTS_DIR=$$(mktemp -d) ./bin/kyber > e2e.log 2>&1 & pid=$$!; \
	sleep 1; KYBER_E2E_URL=http://localhost:18080 npm --prefix web run e2e; rc=$$?; kill $$pid; exit $$rc

docker:
	docker build -t kyber:dev .

docs: ## build the documentation site (VitePress) into site/.vitepress/dist
	npm --prefix site ci --no-audit --no-fund
	npm --prefix site run build

audit: ## dependency and static security checks (govulncheck needs vuln.go.dev)
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	go run github.com/securego/gosec/v2/cmd/gosec@v2.22.4 -quiet -exclude=G101,G104,G115,G304,G505 -exclude-dir=web -exclude-dir=site ./...
	npm --prefix web audit --omit=dev
	npm --prefix site audit --omit=dev

ship: lint web web-test test cover-strict build e2e docs audit ## release gate: everything must pass before tagging (docs/operations/release.md)
	@echo "ship gate passed"
