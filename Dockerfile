# Build stage
FROM golang:1.23-bookworm AS builder
WORKDIR /app

# Copy dependency definitions
COPY go.mod go.sum ./
RUN go mod download

# Install the Playwright driver for Go explicitly so it lands in /go/bin
RUN go install github.com/playwright-community/playwright-go/cmd/playwright@latest
RUN /go/bin/playwright install --with-deps

# Build the binary
COPY . .
RUN go build -o go_scrap .

# Runtime stage
# Use the official Playwright image which includes necessary browsers and dependencies
FROM mcr.microsoft.com/playwright:v1.48.0-jammy
WORKDIR /app

COPY --from=builder /app/go_scrap .
# Copy the Playwright driver and browsers from the builder/cache if needed, 
# but usually the Go library looks in specific paths. 
# To be safe, we rely on the base image browsers and ensure the driver is compatible.
# The simplest way to ensure the driver exists is to copy the cache or run install again (fast if cached).
COPY --from=builder /go/bin/playwright /usr/local/bin/

# Default entrypoint
ENTRYPOINT ["./go_scrap"]