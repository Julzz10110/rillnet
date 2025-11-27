FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy modules first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux go build -o rillnet-signal ./cmd/signal

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary
COPY --from=builder /app/rillnet-signal .

# Create config directory and copy configs
RUN mkdir -p configs
COPY --from=builder /app/configs/config.yaml ./configs/

EXPOSE 8081

CMD ["./rillnet-signal"]