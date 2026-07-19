FROM golang:1.25-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src/services/realtime

COPY services/realtime/go.mod services/realtime/go.sum ./
RUN go mod download

COPY services/realtime ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags pgx5driver -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=build /out/gateway /usr/local/bin/gateway

RUN adduser -D -g '' -u 1001 chess404
USER chess404

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/readyz || exit 1

CMD ["/usr/local/bin/gateway"]
