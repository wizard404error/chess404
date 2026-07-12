FROM golang:1.25-bookworm AS build

WORKDIR /src/services/realtime

COPY services/realtime/go.mod services/realtime/go.sum ./
RUN go mod download -x

COPY services/realtime ./

RUN go vet ./cmd/matchmaking-service/... && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags pgx5driver -ldflags="-s -w" -o /out/matchmaking-service ./cmd/matchmaking-service

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl

COPY --from=build /out/matchmaking-service /usr/local/bin/matchmaking-service

RUN adduser -D -g '' -u 1001 chess404
USER chess404

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/readyz || exit 1

CMD ["/usr/local/bin/matchmaking-service"]
