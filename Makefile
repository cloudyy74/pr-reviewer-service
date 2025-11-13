run:
	go run cmd/pr-reviewer-service/main.go --config_path ./config/local.yml

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...
