FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Install swag and generate Swagger docs
RUN go install github.com/swaggo/swag/cmd/swag@latest
RUN /go/bin/swag init -g cmd/server.go -o docs

# Build the server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Create data directory for mock auth persistence
RUN mkdir -p /app/data && chown -R appuser:appgroup /app

# Copy the binary from builder
COPY --from=builder /app/server .

# Switch to non-root user
USER appuser

EXPOSE 8080

CMD ["./server"]
