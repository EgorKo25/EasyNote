FROM golang:1.25.7-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/easynote .

FROM alpine:3.22

RUN addgroup -S easynote && adduser -S -G easynote easynote

WORKDIR /app

COPY --from=build /out/easynote /app/easynote
COPY config ./config

RUN mkdir -p /app/logs && chown -R easynote:easynote /app

USER easynote

ENTRYPOINT ["/app/easynote"]
