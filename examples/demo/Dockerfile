# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod files from root
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the demo
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /demo ./examples/demo

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /demo .

EXPOSE 3000

CMD ["./demo"]
