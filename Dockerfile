# syntax=docker/dockerfile:1

########################################
# Builder stage
########################################
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Enable Go modules and leverage caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd ./cmd
COPY internal ./internal
COPY data ./data

# Disable CGO for static binary
ENV CGO_ENABLED=0
RUN go build -o radar ./cmd/api

########################################
# Runtime stage
########################################
FROM alpine:3.20

RUN addgroup -S radar && adduser -S radar -G radar

WORKDIR /app

# Copy the compiled binary and runtime assets
COPY --from=builder /app/radar ./radar
COPY --from=builder /app/data ./data

# Ensure certificates are present for outbound HTTPS calls
RUN apk add --no-cache ca-certificates

USER radar

EXPOSE 8080

ENV RADAR_LISTEN_ADDR=:8080 \
    RADAR_STATIC_DATA=data/sample_news.json

ENTRYPOINT ["/app/radar"]
