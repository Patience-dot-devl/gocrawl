BINARY := gocrawl
PKG := ./...

.PHONY: build test vet lint run tidy clean install

build:
	go build -o $(BINARY) ./cmd/gocrawl

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
