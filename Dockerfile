# Build stage
FROM golang:1.22-alpine3.21 AS builder
WORKDIR /app
COPY . .
RUN apk add --no-cache make gcc libc-dev curl
RUN go build -o main cmd/main.go

# Run stage
FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/Makefile .
COPY db/migrations ./db/migrations
COPY .env .
COPY start.sh .

RUN chmod +x /app/start.sh

RUN apk add --no-cache make postgresql-client

EXPOSE 8080 9000

ENTRYPOINT ["/app/start.sh"]
CMD ["/app/main"]