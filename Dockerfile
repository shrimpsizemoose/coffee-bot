# build
FROM golang:1.24-alpine AS builder
LABEL org.opencontainers.image.source=https://github.com/shrimpsizemoose/coffee-bot

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bot

# run
FROM alpine:3.19

COPY --from=builder /bot /bot
RUN mkdir /config
RUN adduser -D bot
USER bot
ENTRYPOINT ["/bot"]
