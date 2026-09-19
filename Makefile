APP=magnit_bot
COMPOSE_FILE=docker-compose.dev.yml
GO_CMD=go run ./cmd/main

.PHONY: run go test build docker-build up down logs

run go:
	$(GO_CMD)

test:
	go test ./...

build:
	go build -o bin/$(APP) ./cmd/main

docker-build:
	docker build -t $(APP):latest .

up:
	docker compose -f $(COMPOSE_FILE) up --build -d

down:
	docker compose -f $(COMPOSE_FILE) down

logs:
	docker compose -f $(COMPOSE_FILE) logs -f bot
