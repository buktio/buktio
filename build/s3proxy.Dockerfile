# syntax=docker/dockerfile:1
# buktio-s3proxy image: a thin counting reverse proxy in front of the Garage S3
# plane (per-key traffic/egress metering). Build context is the repo root.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" \
    -o /out/buktio-s3proxy ./cmd/buktio-s3proxy

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10002 buktio
COPY --from=build /out/buktio-s3proxy /usr/local/bin/buktio-s3proxy
USER buktio
EXPOSE 3900
ENTRYPOINT ["/usr/local/bin/buktio-s3proxy"]
