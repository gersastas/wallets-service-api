.PHONY: run test lint up down db-shell db-reset

run:
	go run ./cmd/service/main.go

test:
	go test ./tests -v

lint:
	golangci-lint run

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