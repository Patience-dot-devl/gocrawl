BINARY := gocrawl
PKG := ./...
# Same golangci-lint version as .github/workflows/ci.yml; bump both together. `go run` builds it
# with this repo's Go toolchain, so a stale brew-installed binary can't break `make lint`.
GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0

.PHONY: build test vet lint check docs run tidy clean install web-build dev docker

# Plain `go build` embeds the committed placeholder UI (see internal/webserver/assets.go) and
# needs no Node. Run `make web-build` first to embed the real web UI in the binary.
# `vite build` always writes an assets/ folder next to index.html; the committed placeholder
# has none, so its absence means this build embeds the placeholder, not the real UI. When the
# real UI is there but web/ changed after it was built, the binary embeds a stale UI.
build:
	go build -o $(BINARY) ./cmd/gocrawl
	@if [ ! -d internal/webserver/webui/dist/assets ]; then \
		echo "warning: $(BINARY) embeds the placeholder web UI; run 'make web-build' first to embed the real one (or 'make dev' for frontend work)" >&2; \
	elif [ -n "$$(find web/src web/index.html -newer internal/webserver/webui/dist/index.html 2>/dev/null | head -n 1)" ]; then \
		echo "warning: $(BINARY) embeds a web UI that is older than web/; run 'make web-build' and rebuild to embed the current one" >&2; \
	fi

# Builds the web/ frontend straight into internal/webserver/webui/dist/ (see web/vite.config.ts),
# which the next `make build` embeds. Requires Node/npm.
web-build:
	cd web && npm ci && npm run build

# Frontend development with hot reload: runs `gocrawl serve` on DEV_ADDR and `npm run dev` on
# :5173, whose proxy (web/vite.config.ts) sends /api/* to DEV_ADDR through GOCRAWL_API. Open
# http://localhost:5173. Ctrl-C, or either process exiting, stops both. Installs
# web/node_modules on first use.
DEV_ADDR ?= 127.0.0.1:8080
# Plain POSIX sh with no `wait -n`, so it also runs on macOS, whose /bin/bash is 3.2: poll both
# PIDs and stop the other process when one exits. `exec` makes $$ui the npm process itself, not
# a subshell, so the kill reaches it.
dev:
	@[ -d web/node_modules ] || (cd web && npm ci)
	go build -o $(BINARY) ./cmd/gocrawl
	@./$(BINARY) serve --addr $(DEV_ADDR) & api=$$!; \
	(cd web && GOCRAWL_API=http://$(DEV_ADDR) exec npm run dev) & ui=$$!; \
	trap 'kill $$api $$ui 2>/dev/null' INT TERM EXIT; \
	while kill -0 $$api 2>/dev/null && kill -0 $$ui 2>/dev/null; do sleep 1; done

# Builds the container image from the multi-stage Dockerfile (web UI, then a static binary).
DOCKER_IMAGE ?= gocrawl:dev
docker:
	docker build -t $(DOCKER_IMAGE) .

test:
	go test $(PKG)

vet:
	go vet $(PKG)

lint:
	$(GOLANGCI_LINT) run

# Regenerates the generated parts of docs/: each document's depends_on and sources
# frontmatter, and the index in docs/README.md. Run after adding a doc or changing its links,
# title, scope, status, or updated date.
docs:
	go test ./internal/repocheck -run 'TestDocs' -update

# Everything CI runs on the Go code, in one command. AGENTS.md asks agents to run this before
# reporting work as done; the tests include the docs-drift checks in internal/repocheck.
check:
	@unformatted="$$(gofmt -l cmd internal)"; \
	if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	go mod verify
	go vet $(PKG)
	go build $(PKG)
	go test -race $(PKG)
	$(GOLANGCI_LINT) run

run: build
	./$(BINARY) crawl https://example.com --depth 1

tidy:
	go mod tidy

install:
	go install ./cmd/gocrawl

clean:
	rm -f $(BINARY)
	go clean
