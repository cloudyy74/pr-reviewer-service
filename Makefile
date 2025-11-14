env:  ##@Environment Create .env file with variables
	cp .env.example .env

run:  ##@Application Run application server
	go run cmd/pr-reviewer-service/main.go --config_path ./config/local.yml

lint:  ##@Code Check code with golangci-lint
	golangci-lint run ./...

fmt:  ##@Code Reformat code with gofmt and goimports
	go fmt ./...
	goimports -w .

test:  ##@Testing Test application with go test
	go test ./...
