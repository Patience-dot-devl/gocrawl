# syntax=docker/dockerfile:1

# Container image for `gocrawl serve` (and every other subcommand) with the real web UI
# embedded. Three stages: build the web/ frontend, compile a static binary that embeds it, and
# copy that binary into a distroless image. Build with `make docker` or `docker build .`.
#
# The image has no browser, so `--render headless` and "Log in with browser…" are not available
# in it; raw-HTML crawling (the default) works.

# The frontend and Go stages run on the build machine's platform and cross-compile, so a
# multi-arch build does not run npm or the Go compiler under emulation.
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# vite.config.ts writes to ../internal/webserver/webui/dist, where assets.go embeds it from.
RUN mkdir -p ../internal/webserver/webui && npm run build

FROM --platform=$BUILDPLATFORM golang:1.27 AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/webserver/webui/dist ./internal/webserver/webui/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/gocrawl ./cmd/gocrawl
# The state directory (crawl store, GUI settings, header profiles) must exist and belong to the
# nonroot user, or a volume mounted on it starts out owned by root and unwritable.
RUN mkdir -p /out/state/.gocrawl

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/gocrawl /usr/local/bin/gocrawl
COPY --from=build --chown=65532:65532 /out/state/.gocrawl /home/nonroot/.gocrawl
# Crawl history (~/.gocrawl/crawls), GUI settings and header profiles. Mount a volume here to
# keep them across container restarts.
VOLUME ["/home/nonroot/.gocrawl"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gocrawl"]
# A container must listen on all interfaces to be reachable through a published port. The web
# API has no authentication, so publish it on loopback only (docker run -p 127.0.0.1:8080:8080)
# or put something that authenticates in front of it; see docs/web.md#exposure.
CMD ["serve", "--addr", "0.0.0.0:8080"]
