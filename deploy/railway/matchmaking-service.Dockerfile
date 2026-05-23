FROM golang:1.25-bookworm AS build

WORKDIR /src/services/realtime

COPY services/realtime/go.mod services/realtime/go.sum ./
RUN go mod download

COPY services/realtime ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/matchmaking-service ./cmd/matchmaking-service

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=build /out/matchmaking-service /usr/local/bin/matchmaking-service

RUN adduser -D -g '' -u 1001 nobody
USER nobody

EXPOSE 8080

CMD ["/usr/local/bin/matchmaking-service"]
