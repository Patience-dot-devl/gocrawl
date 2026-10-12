BINARY := gocrawl
PKG := ./...
# Same golangci-lint version as .github/workflows/ci.yml; bump both together. `go run` builds it
# with this repo's Go toolchain, so a stale brew-installed binary can't break `make lint`.
GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0

.PHONY: build test vet lint check run tidy clean install web-build

# Plain `go build` embeds the committed placeholder UI (see internal/webserver/assets.go) and
# needs no Node. Run `make web-build` first to embed the real web UI in the binary.
build:
	go build -o $(BINARY) ./cmd/gocrawl

# Builds the web/ frontend straight into internal/webserver/webui/dist/ (see web/vite.config.ts),
# which the next `make build` embeds. Requires Node/npm.
web-build:
	cd web && npm ci && npm run build

test:
	go test $(PKG)

vet:
	go vet $(PKG)

lint:
	$(GOLANGCI_LINT) run

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
