# syntax=docker/dockerfile:1
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/go/build \
    CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /openstat ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /openstat .

# Запуск от root — volume /app/data иначе может иметь проблемы с правами
EXPOSE 8080
ENTRYPOINT ["./openstat"]
CMD ["-status=/var/log/openvpn/status.log", "-interval=60s"]
