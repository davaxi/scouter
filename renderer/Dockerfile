# syntax=docker/dockerfile:1.7
#
# JS renderer (go-rod + Chromium). Published as davaxi/scouter-renderer.
# Stage 1 cross-compiles the Go binary on the build host's native arch.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /src
# Resolve dependencies from go.sum (reproducible) instead of `go mod tidy`,
# which could silently pull newer versions at build time.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY main.go ./
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/renderer .

# Runtime - Debian with Chromium (the chromium package pulls its own libs:
# nss, atk, cups, drm, gbm, asound…).
FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends chromium ca-certificates fonts-liberation \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/renderer /app/renderer
EXPOSE 3000
# Rod utilisera Chromium
ENV ROD_BROWSER=/usr/bin/chromium
CMD ["/app/renderer"]
