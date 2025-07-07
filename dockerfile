
# Use the official Golang image to build the application
# Using a specific version is a good practice
FROM golang:1.21-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the Go application
# -o /app/server creates the binary named 'server'
# CGO_ENABLED=0 is important for creating a static binary
# -ldflags="-w -s" strips debug information to make the binary smaller
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/server -ldflags="-w -s" .

# ---
# Create the final, small image
# Using a distroless image for a smaller and more secure final image
FROM gcr.io/distroless/static-debian11

# Set the working directory
WORKDIR /

# Copy the built binary from the builder stage
COPY --from=builder /app/server /server

# The command to run when the container starts
ENTRYPOINT ["/server"]
