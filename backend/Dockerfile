# Build stage — compile a static binary, discard the toolchain afterward.
FROM golang:1.27-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Runtime stage — minimal image, non-root user.
FROM alpine:3.24

RUN apk add --no-cache ca-certificates wget \
  && adduser -D -u 10001 -H -s /sbin/nologin appuser

COPY --from=builder /out/server /usr/local/bin/server

USER appuser
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/livez >/dev/null || exit 1

CMD ["/usr/local/bin/server"]
