.PHONY: test lint run build docker cover web web-test e2e

COVER_MIN ?= 85

test: ## set KYBER_TEST_DATABASE_URL to include Postgres integration tests
	go test -race -coverpkg=./internal/...,./web/... -coverprofile=coverage.out ./...
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/ {sub("%","",$$3); print $$3}'); \
	echo "total coverage: $$total% (min $(COVER_MIN)%)"; \
	awk -v t=$$total -v m=$(COVER_MIN) 'BEGIN { exit (t+0 < m+0) }'

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

e2e: build ## runs Playwright against a fresh in-memory server on :18080
	@KYBER_ADDR=:18080 KYBER_COOKIE_SECURE=false ./bin/kyber > e2e.log 2>&1 & pid=$$!; \
	sleep 1; KYBER_E2E_URL=http://localhost:18080 npm --prefix web run e2e; rc=$$?; kill $$pid; exit $$rc

docker:
	docker build -t kyber:dev .
