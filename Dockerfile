# Pull golang alpine to build binary
FROM golang:alpine as builder

# Update the package list and install the required packages
RUN apk add --no-cache gcc musl-dev make bash

# Set the working directory
WORKDIR /app

# Copy the source code and Makefile into the container
COPY .. .

RUN mkdir -p bin

# Build the application
# RUN make build-app

RUN CGO_ENABLED=1 go build -a -o bin/nyx -ldflags="-w -s" ./cmd/nyx/
RUN CGO_ENABLED=1 go build -a -o bin/client -ldflags="-w -s" ./cmd/client/

# Use a lightweight base image for the final stage
FROM alpine:latest

# Copy the binary from the builder stage
COPY --from=builder /app/bin/nyx /bin/nyx
COPY --from=builder /app/bin/client /bin/client
COPY --from=builder /app/docker-entrypoint.sh /bin/docker-entrypoint.sh

RUN chmod +x /bin/docker-entrypoint.sh

RUN mkdir -p /nyx/file
VOLUME /nyx/file

# Run app and expose api and metrics ports

# API
EXPOSE 4001 4001

# Metrics
# EXPOSE 7300

ENTRYPOINT ["/bin/docker-entrypoint.sh"]

# Run app
CMD ["/bin/nyx"]

