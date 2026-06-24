# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/notify ./cmd/notify

# Final stage — alpine instead of scratch for k8s runner compatibility
FROM alpine:3.20

RUN apk --no-cache add ca-certificates

COPY --from=builder /bin/notify /notify

ENTRYPOINT ["/notify"]