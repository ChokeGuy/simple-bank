#Build stage
FROM golang:1.22-alpine3.21 AS builder
WORKDIR /app
COPY . .
RUN go build -o main cmd/main.go

#Run stage
FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/main .

EXPOSE 8080
CMD ["/app/main"]