# syntax=docker/dockerfile:1.7
#
# Go crawl worker (crawler-go). Published as davaxi/scouter-crawler.
# Stage 1 cross-compiles a static binary on the build host's native arch (no
# QEMU even for multi-arch images). Stage 2 = minimal runtime with CA certs.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/scouter-crawler ./cmd/scouter-crawler

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 scouter
COPY --from=build /out/scouter-crawler /usr/local/bin/scouter-crawler
USER scouter
ENTRYPOINT ["/usr/local/bin/scouter-crawler"]
