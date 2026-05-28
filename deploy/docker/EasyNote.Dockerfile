FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG APP_PACKAGE=./cmd/easynote
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/easy-note ${APP_PACKAGE}

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata && \
    mkdir -p /app/logs

COPY --from=builder /out/easy-note /app/easy-note
COPY config.yaml /app/config.yaml

ENTRYPOINT ["/app/easy-note"]
CMD ["--config", "/app/config.yaml"]