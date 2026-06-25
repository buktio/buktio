# syntax=docker/dockerfile:1
# buktio API image. Build context is the repo root.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/buktio-api ./apps/api/cmd/server

# Alpine (not distroless) so the compose wget healthcheck works.
# postgresql-client provides pg_dump for the metadata-backup feature (v2-M7).
FROM alpine:3.20
RUN apk add --no-cache wget ca-certificates postgresql16-client \
 && adduser -D -u 10001 buktio \
 && mkdir -p /var/lib/buktio/backups && chown buktio /var/lib/buktio/backups
COPY --from=build /out/buktio-api /usr/local/bin/buktio-api
LABEL org.opencontainers.image.source="https://github.com/buktio/buktio"
USER buktio
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/buktio-api"]
