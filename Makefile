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
	go test -v ./...

compose-up:  ##@Docker Run application with docker-compose
	docker compose up

compose-down:  ##@Docker Shutdown application with docker-compose
	docker compose down --remove-orphans

compose-test-up:  ##@Docker Run tests with docker-compose
	docker compose -f docker-compose.test.yml up --abort-on-container-exit --exit-code-from tests

compose-test-down:  ##@Docker Shutdown tests containers with docker-compose
	docker compose -f docker-compose.test.yml down -v --remove-orphans

