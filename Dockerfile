FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o bin/pr-reviewer-service ./cmd/pr-reviewer-service

FROM alpine:3.21
COPY --from=builder ./app/bin ./bin
COPY --from=builder ./app/config ./config

ENTRYPOINT ["./bin/pr-reviewer-service"]
CMD ["--config_path", "/config/docker.yml"]