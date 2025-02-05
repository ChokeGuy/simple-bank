# Build stage
FROM golang:1.22-alpine3.21 AS builder
WORKDIR /app
COPY . .
RUN apk add --no-cache make gcc libc-dev curl
RUN go build -o main cmd/main.go
RUN curl -L https://github.com/golang-migrate/migrate/releases/download/v4.18.1/migrate.linux-amd64.tar.gz | tar xvz
RUN mv migrate ./migrate

# Run stage
FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/migrate ./migrate
COPY --from=builder /app/Makefile .
COPY db/migrations ./migrations
COPY .env .
COPY start.sh .

RUN apk add --no-cache make postgresql-client

EXPOSE 8080

ENTRYPOINT ["/app/start.sh"]
CMD ["/app/main"]