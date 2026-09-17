# Stage 1: Build the Go binary
FROM golang:alpine AS builder

ENV GOTOOLCHAIN=auto

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o diagram-backend main.go

# Stage 2: Final lightweight runtime
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/diagram-backend /app/diagram-backend
COPY --from=builder /app/.env.example /app/.env.example

EXPOSE 8080

CMD ["/app/diagram-backend"]
