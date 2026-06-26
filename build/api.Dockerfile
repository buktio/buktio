# syntax=docker/dockerfile:1
# buktio API image — a SINGLE binary with the web panel embedded (internal/webui),
# so there is no separate Node web container. Build context is the repo root.
#
# Multi-arch safe: the web static export runs on the native $BUILDPLATFORM (so
# Next.js/SWC never runs under QEMU emulation, which used to SIGILL on arm64), then
# Go cross-compiles the server to the target arch and embeds the export.

# Stage 1 — web static export, pinned to the build host's native arch.
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /web
RUN corepack enable
COPY apps/web/package.json apps/web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY apps/web/ ./
ENV NEXT_TELEMETRY_DISABLED=1
# Optional public read-only-demo hint shown on the login page. Empty in normal
# builds (no banner); set only for the hosted demo image.
ARG NEXT_PUBLIC_DEMO_EMAIL=""
ARG NEXT_PUBLIC_DEMO_PASSWORD=""
ENV NEXT_PUBLIC_DEMO_EMAIL=$NEXT_PUBLIC_DEMO_EMAIL NEXT_PUBLIC_DEMO_PASSWORD=$NEXT_PUBLIC_DEMO_PASSWORD
RUN pnpm build

# Stage 2 — Go build on the native builder, cross-compiled to the target arch.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG TARGETOS TARGETARCH VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Overlay the freshly built UI onto internal/webui/dist (replacing the placeholder).
COPY --from=web /web/out/. ./internal/webui/dist/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" -o /out/buktio-api ./apps/api/cmd/server

# Stage 3 — runtime. Alpine (not distroless) so the compose wget healthcheck works.
# postgresql-client provides pg_dump for the metadata-backup feature.
FROM alpine:3.20
RUN apk add --no-cache wget ca-certificates postgresql16-client \
 && adduser -D -u 10001 buktio \
 && mkdir -p /var/lib/buktio/backups && chown buktio /var/lib/buktio/backups
COPY --from=build /out/buktio-api /usr/local/bin/buktio-api
LABEL org.opencontainers.image.source="https://github.com/buktio/buktio"
USER buktio
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/buktio-api"]
