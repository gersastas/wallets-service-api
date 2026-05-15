.PHONY: run test lint up down build docker-build db-shell db-reset

run:
	go run ./cmd/service/main.go

build:
	go build -o bin/server ./cmd/service/main.go

test:
	go test ./... -v

lint:
	golangci-lint run

docker-build:
	docker build -t wallets-service .

up:
	docker compose up -d

down:
	docker compose down

db-shell:
	docker exec -it postgres-wallet psql -U postgres -d wallet_db

db-reset:
	docker compose down -v
	docker compose up -d
	@echo "Waiting for PostgreSQL to start..."
	@sleep 3
	@echo "Database reset complete!"
