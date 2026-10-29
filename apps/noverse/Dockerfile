# syntax=docker/dockerfile:1.7
#
# Multi-stage image for the hosted (Dokploy/Traefik) deployment:
#   1. web  — Vite build of the SPA into the exact directory the Go binary embeds
#   2. go   — static binary (CGO off) with that dist inside it
#   3. run  — Alpine + CA/tzdata + ffmpeg/ffprobe, non-root, no shell toolkit
#
# The runtime deliberately contains no Node, no npm cache, no test data, no
# ~/.env or host library. Only /var/lib/noverse/blobs, /var/cache/noverse and
# the config dir are writable by the non-root user.

# ── 1. web ────────────────────────────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM node:22-alpine AS web

WORKDIR /src/web

# Workspace manifests first so the dependency layer caches independently of
# source edits. `@noverse/ui` is a file workspace under web/packages/ui.
COPY web/package.json web/package-lock.json ./
COPY web/packages/ui/package.json packages/ui/package.json
RUN npm ci --no-audit --no-fund

COPY web/ ./
# vite.config.ts sets build.outDir = ../internal/entry/web/dist, i.e.
# /src/internal/entry/web/dist — the directory `//go:embed all:dist` expects.
RUN npm run build

# ── 2. go ─────────────────────────────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /src

ENV CGO_ENABLED=0 GOWORK=off

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=
ARG DATE=

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Web build output must exist before `go build`: the embed refuses a missing
# dist, which is exactly the packaging failure this stage would otherwise hide.
COPY --from=web /src/internal/entry/web/dist ./internal/entry/web/dist

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /out/ainovel-cli \
    ./cmd/ainovel-cli

# ── 3. runtime ────────────────────────────────────────────────────────────────
FROM alpine:3.22

# ffmpeg ships ffprobe; both are required by the hosted audio/export path.
# ca-certificates/timezone are needed by S3/TLS and scheduled jobs.
RUN apk add --no-cache ca-certificates tzdata ffmpeg \
    && addgroup -S -g 10001 noverse \
    && adduser -S -u 10001 -G noverse -h /home/noverse noverse \
    && mkdir -p /var/lib/noverse/blobs /var/cache/noverse /home/noverse/.ainovel \
    && chown -R noverse:noverse /var/lib/noverse /var/cache/noverse /home/noverse

COPY --from=builder /out/ainovel-cli /usr/local/bin/ainovel-cli

ENV HOME=/home/noverse \
    HTTP_ADDR=0.0.0.0:8080 \
    TMPDIR=/var/cache/noverse

WORKDIR /home/noverse
USER 10001:10001

EXPOSE 8080

# wget is BusyBox's, installed with Alpine — no curl to add, no probe that can
# silently be absent. Liveness only; readiness stays behind /readyz.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${HEALTHCHECK_PORT:-8080}/healthz" || exit 1

ENTRYPOINT ["ainovel-cli"]
CMD ["serve"]
