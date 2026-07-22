FROM golang:1.25.7-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG APP_TARGET=./cmd/backend

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/easy-note ${APP_TARGET}

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata && \
    mkdir -p /app/logs

COPY --from=builder /out/easy-note /app/easy-note
COPY config ./config

EXPOSE 8080

ENTRYPOINT ["/app/easy-note"]
