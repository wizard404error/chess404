FROM golang:1.25-bookworm AS build

WORKDIR /src/services/realtime

COPY services/realtime/go.mod services/realtime/go.sum ./
RUN go mod download

COPY services/realtime ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/platform-service ./cmd/platform-service

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=build /out/platform-service /usr/local/bin/platform-service

EXPOSE 8083

CMD ["/usr/local/bin/platform-service"]
