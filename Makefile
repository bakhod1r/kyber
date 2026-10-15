.PHONY: test cover-strict lint run build docker cover web web-test e2e e2e-rooms e2e-egress e2e-replicas screenshots docs audit ship

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

LIVEKIT_SERVER ?= livekit-server
e2e-rooms: build ## runs the media-server e2e (S99) against livekit-server --dev and a server on :18081
	@$(LIVEKIT_SERVER) --dev --bind 127.0.0.1 > livekit.log 2>&1 & lk=$$!; \
	KYBER_ADDR=:18081 KYBER_COOKIE_SECURE=false KYBER_LIVEKIT_URL=ws://localhost:7880 KYBER_LIVEKIT_KEY=devkey \
	KYBER_LIVEKIT_SECRET=secret ./bin/kyber > e2e-rooms.log 2>&1 & pid=$$!; \
	for i in $$(seq 50); do curl -fs http://localhost:7880 > /dev/null && break; sleep 0.2; done; \
	sleep 1; KYBER_E2E_ROOM_URL=http://localhost:18081 npm --prefix web run e2e -- room.spec.ts; rc=$$?; kill $$pid $$lk; exit $$rc

e2e-replicas: build ## runs replicas.spec.ts (S58, S105) against two servers on KYBER_E2E_DATABASE_URL
	@test -n "$(KYBER_E2E_DATABASE_URL)" || (echo "set KYBER_E2E_DATABASE_URL (an empty database)" && exit 1)
	@KYBER_DATABASE_URL="$(KYBER_E2E_DATABASE_URL)" KYBER_ADDR=:18091 KYBER_COOKIE_SECURE=false KYBER_ATTACHMENTS_DIR=$$(mktemp -d) ./bin/kyber > e2e-replica1.log 2>&1 & a=$$!; \
	for i in $$(seq 100); do curl -fs http://localhost:18091/readyz > /dev/null && break; sleep 0.2; done; \
	KYBER_DATABASE_URL="$(KYBER_E2E_DATABASE_URL)" KYBER_ADDR=:18092 KYBER_COOKIE_SECURE=false KYBER_ATTACHMENTS_DIR=$$(mktemp -d) ./bin/kyber > e2e-replica2.log 2>&1 & b=$$!; \
	for i in $$(seq 100); do curl -fs http://localhost:18092/readyz > /dev/null && break; sleep 0.2; done; \
	KYBER_E2E_URL=http://localhost:18091 KYBER_E2E_URL2=http://localhost:18092 npm --prefix web run e2e -- replicas.spec.ts; rc=$$?; kill $$a $$b; exit $$rc

EGRESS_IMAGE ?= livekit/egress:v1.9.0
e2e-egress: build ## runs the room e2e with server-side recording (S104): Redis and LiveKit Egress in Docker
	@rec=$$(mktemp -d); chmod 777 $$rec; \
	docker run -d --rm --name kyber-e2e-redis --network host redis:7-alpine > /dev/null; \
	$(LIVEKIT_SERVER) --dev --bind 127.0.0.1 --redis-host localhost:6379 > livekit.log 2>&1 & lk=$$!; \
	docker run -d --rm --name kyber-e2e-egress --network host --cap-add SYS_ADMIN -v $$rec:/out \
	  -e EGRESS_CONFIG_BODY="$$(printf 'api_key: devkey\napi_secret: secret\nws_url: ws://localhost:7880\nredis:\n  address: localhost:6379\n')" \
	  $(EGRESS_IMAGE) > /dev/null; \
	for i in $$(seq 120); do docker logs kyber-e2e-egress 2>&1 | grep -q "service ready" && break; sleep 1; done; \
	KYBER_ADDR=:18082 KYBER_COOKIE_SECURE=false KYBER_LIVEKIT_URL=ws://localhost:7880 KYBER_LIVEKIT_KEY=devkey \
	KYBER_LIVEKIT_SECRET=secret KYBER_LIVEKIT_RECORDINGS_DIR=$$rec ./bin/kyber > e2e-egress.log 2>&1 & pid=$$!; \
	sleep 2; KYBER_E2E_EGRESS=1 KYBER_E2E_ROOM_URL=http://localhost:18082 npm --prefix web run e2e -- room.spec.ts; rc=$$?; \
	left=$$(ls $$rec); [ -z "$$left" ] || { echo "recordings left behind: $$left"; rc=1; }; \
	kill $$pid $$lk; docker rm -f kyber-e2e-egress kyber-e2e-redis > /dev/null; exit $$rc

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
