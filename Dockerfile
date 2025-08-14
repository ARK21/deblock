FROM golang:1.24.1 as builder

WORKDIR /src
COPY . .
RUN go build -o /out/watcher ./cmd/watcher

FROM gcr.io/distroless/base-debian12

COPY --from builder /out/watcher /watcher/

ENTRYPOINT ["/watcher"]