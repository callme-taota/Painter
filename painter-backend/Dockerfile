# Build stage for Golang and Node.js
FROM golang:1.22 AS go-build
LABEL maintainer="Taota"
LABEL description="Find me http://www.callmetaota.fun/"

WORKDIR /app

# Copy Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy entire application source code
COPY . .

# Set environment variables for Go build
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Build the Go binary
RUN go build -x -o painter .

# Build stage for Node.js (Vue)
FROM node:20 AS node-build

WORKDIR /app/server/static/webroot

# Copy package.json and install dependencies
COPY server/static/webroot/package*.json ./
RUN npm install

# Copy the rest of the frontend source code and build
COPY server/static/webroot ./
RUN npm run build

# Final stage - slim production image
FROM debian:buster-slim

WORKDIR /root/

# Create necessary directories
RUN mkdir -p "/root/conf" \
    && mkdir -p "/root/tolog/logs" \
    && mkdir -p "/root/server/static/upload" \
    && mkdir -p "/root/server/static/webroot/dist"

# Copy Go binary from go-build stage
COPY --from=go-build /app/painter .

# Copy Vue build output from node-build stage
COPY --from=node-build /app/server/static/webroot/dist ./server/static/webroot/dist

# Copy configuration files
COPY --from=go-build /app/conf/conf.json ./conf

# Ensure the binary is executable
RUN chmod +x ./painter

# Expose the application's port
EXPOSE 3003

# Start the Go application
CMD ["./painter"]
