# Build stage
FROM golang:1.21-alpine AS builder

# Install git and ca-certificates for downloading modules
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o hydra-route \
    ./cmd/hydra-route

# Runtime stage
FROM scratch

# Import from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary
COPY --from=builder /workspace/hydra-route /usr/local/bin/hydra-route

# Create non-root user
USER 1001

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/hydra-route"] 