# --- build stage ---
FROM golang:1.24-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates tzdata

# Leverage caching
COPY go.mod go.sum ./
RUN go mod download

# Copy sources
COPY . .

# Static-ish build (no CGO needed for watermill)
ENV CGO_ENABLED=0 GOFLAGS="-trimpath"
RUN go build -o /out/watcher ./cmd/watcher

# --- runtime stage ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -g '' app
USER app
WORKDIR /app
COPY --from=builder /out/watcher /app/watcher
COPY --chown=app:app ./addresses.csv /app/addresses.csv
# data dir for checkpoints by default
RUN mkdir -p /app/data
EXPOSE 8080
ENTRYPOINT ["/app/watcher"]