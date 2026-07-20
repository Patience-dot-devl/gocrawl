BINARY := gocrawl
PKG := ./...

.PHONY: build test vet lint run tidy clean install web-build

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
	golangci-lint run

run: build
	./$(BINARY) crawl https://example.com --depth 1

tidy:
	go mod tidy

install:
	go install ./cmd/gocrawl

clean:
	rm -f $(BINARY)
	go clean
