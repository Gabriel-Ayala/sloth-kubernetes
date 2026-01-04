# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE} -X main.builtBy=docker" \
    -o /sloth ./main.go

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    openssh-client \
    wireguard-tools \
    curl \
    bash

# Copy binary from builder
COPY --from=builder /sloth /usr/local/bin/sloth

# Create non-root user
RUN adduser -D -g '' sloth
USER sloth

# Set working directory
WORKDIR /workspace

ENTRYPOINT ["sloth"]
CMD ["--help"]
