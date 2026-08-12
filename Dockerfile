FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY backend-go/go.mod backend-go/go.sum ./
RUN go mod download
COPY backend-go/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/main .
EXPOSE 10000
CMD ["./main"]