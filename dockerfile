# Builder stage
FROM golang:1.24-bullseye AS builder

WORKDIR /app

# Install build dependencies needed for CGO (libsqlite3-dev as example)
RUN apt-get update && apt-get install -y build-essential libsqlite3-dev

# Copy Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Build the binary with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /app/server -ldflags="-w -s -linkmode=external -extldflags=-ldl" .

# Final stage: distroless image that supports dynamic libraries
FROM gcr.io/distroless/cc-debian11

WORKDIR /

# Copy the binary from the builder stage
COPY --from=builder /app/server /server

# Run the binary
ENTRYPOINT ["/server"]
