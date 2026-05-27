FROM golang:1.25-bookworm AS build

WORKDIR /src/services/realtime

COPY services/realtime/go.mod services/realtime/go.sum ./
RUN go mod download

COPY services/realtime ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/gateway ./cmd/gateway

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=build /out/gateway /usr/local/bin/gateway

RUN adduser -D -g '' -u 1001 chess404
USER chess404

EXPOSE 8080

CMD ["/usr/local/bin/gateway"]
