FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o bin/pr-reviewer-service ./cmd/pr-reviewer-service

FROM alpine:3.21
RUN apk --no-cache add curl
COPY --from=builder ./app/bin ./bin
COPY --from=builder ./app/config ./config


HEALTHCHECK --interval=30s --timeout=1m --start-period=30s --start-interval=10s --retries=2 CMD curl -f http://localhost:8080/ping

ENTRYPOINT ["./bin/pr-reviewer-service"]
