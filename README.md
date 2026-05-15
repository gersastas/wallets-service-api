# Wallets Service API

REST API сервис для управления кошельками и финансовыми операциями.

## Технологии

- **Go** (chi router, logrus, golang-migrate)
- **PostgreSQL** (хранение данных)
- **Docker** (контейнеризация)

## Быстрый старт

```bash
# Запустить всё через Docker
docker compose up -d

# Или запустить только БД и запустить сервер локально
docker compose up -d postgres
go run ./cmd/service/main.go
```

## Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/wallet_db?sslmode=disable` | Строка подключения к БД |
| `HTTP_BIND_ADDR` | `:8080` | Адрес HTTP сервера |

## API

### Health Check

```bash
GET /health
```

### Кошельки

```bash
# Создать кошелёк
POST /wallets
{
  "user_id": "uuid",
  "name": "My Wallet",
  "currency": "USD"
}

# Получить кошелёк
GET /wallets/{id}

# Обновить кошелёк
PUT /wallets/{id}
{"name": "New Name"}

# Удалить кошелёк
DELETE /wallets/{id}

# Список кошельков
GET /wallets?user_id=uuid&limit=10&offset=0
```

### Финансовые операции

```bash
# Пополнение
POST /wallets/{id}/deposit
{
  "amount": 1000,
  "idempotency_key": "unique-key"
}

# Снятие
POST /wallets/{id}/withdraw
{
  "amount": 500,
  "idempotency_key": "unique-key"
}

# Перевод
POST /wallets/transfer
{
  "from_wallet_id": "uuid",
  "to_wallet_id": "uuid",
  "amount": 300,
  "idempotency_key": "unique-key"
}

# История транзакций
GET /wallets/{id}/transactions?limit=10&offset=0
```

## Swagger документация

Файл `swagger.yaml` в корне проекта. Открыть через [Swagger Editor](https://editor.swagger.io).

## Разработка

```bash
# Запустить тесты
make test

# Запустить линтер
make lint

# Собрать бинарник
make build

# Сбросить БД
make db-reset
```