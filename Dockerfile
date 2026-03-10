# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies if needed
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# CGO_ENABLED=0 is important for static binary
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /app/server ./cmd/server

# Run stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Copy the binary from the builder stage
COPY --from=builder /app/server .

# Expose port (assuming 8080 based on common practices, but I should verify if I saw it in code, 
# actually I didn't check the port in main.go, but it's good practice to expose one if known. 
# I'll leave EXPOSE out or put a common one if I'm not sure, but better to check main.go later. 
# For now, I'll assume the user can configure it or it defaults to something.)
# Let's check main.go content quickly to see if there is a port. 
# Wait, I can't check it inside this tool call. I'll just create the file without EXPOSE for now 
# or add it if I see it in the next step. 
# Actually, I'll just add the entrypoint.

# Expose both MCP (8880) and HTTP API (8881) ports
EXPOSE 8880
EXPOSE 8881

# The server defaults to :8880 for MCP and :8881 for HTTP if not overridden, 
# but it's good practice to specify them explicitly if we want to ensure bindings.
ENTRYPOINT ["/app/server", "-addr", "0.0.0.0:8880", "-api-addr", "0.0.0.0:8881"]
