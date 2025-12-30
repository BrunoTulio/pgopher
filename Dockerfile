FROM golang:1.25-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -trimpath \
    -o pgopher ./main.go

RUN apk add --no-cache upx && \
    upx --best --lzma pgopher

FROM alpine:latest

ARG PG_VERSION=18
ENV PG_VERSION=${PG_VERSION}

RUN apk add --no-cache \
    postgresql${PG_VERSION}-client \
    ca-certificates \
    tzdata && \
    rm -rf /var/cache/apk/*

WORKDIR /app

COPY --from=builder /build/pgopher /usr/local/bin/pgopher

RUN adduser -D -u 1000 pgopher && \
    mkdir -p /data/backups && \
    mkdir -p /var/lib/pgopher/metadata && \
    chown -R pgopher:pgopher /data && \
    chown -R pgopher:pgopher /var/lib/pgopher

USER pgopher

WORKDIR /app

ENTRYPOINT ["/usr/local/bin/pgopher"]
CMD ["--help"]
