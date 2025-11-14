# =========
# Builder
# =========
FROM golang:1.23-alpine AS builder

# Install build deps for CGO + SQLite
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Go module files first (for better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build the binary
# CGO must be enabled for go-sqlite3
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o server main.go

# =========
# Runtime
# =========
FROM alpine:3.20

# Install runtime deps: certs + sqlite libraries
RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Copy compiled binary from builder
COPY --from=builder /app/server .

# Directory for SQLite DB
RUN mkdir -p /data

# Default envs (override in docker run/deploy script)
ENV EXPORT_PASSWORD=changeme
ENV DB_PATH=/data/data.db

# Expose HTTP port
EXPOSE 8080

# Default password (override in docker run / docker-compose)
ENV EXPORT_PASSWORD=changeme

# Data file will be data.db in /app
CMD ["./server"]
