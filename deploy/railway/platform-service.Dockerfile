FROM golang:1.25-bookworm AS build

WORKDIR /src/services/realtime

COPY services/realtime/go.mod services/realtime/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download -x

COPY services/realtime ./

RUN --mount=type=cache,target=/go/pkg/mod CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/platform-service ./cmd/platform-service

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=build /out/platform-service /usr/local/bin/platform-service

RUN adduser -D -g '' -u 1001 chess404
USER chess404

EXPOSE 8080

CMD ["/usr/local/bin/platform-service"]
