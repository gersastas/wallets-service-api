-- Создаём ENUM для типов транзакций
CREATE TYPE transaction_type AS ENUM ('deposit', 'withdraw', 'transfer');

-- Создаём таблицу транзакций
CREATE TABLE IF NOT EXISTS transactions (
                                            id UUID PRIMARY KEY,
                                            wallet_id UUID NOT NULL,
                                            type transaction_type NOT NULL,
                                            amount BIGINT NOT NULL,
                                            currency VARCHAR(10) NOT NULL,
    from_wallet_id UUID,
    to_wallet_id UUID,
    description TEXT,
    idempotency_key VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Внешние ключи
    CONSTRAINT fk_transactions_wallet
    FOREIGN KEY (wallet_id)
    REFERENCES wallets(id)
    ON DELETE CASCADE,

    CONSTRAINT fk_transactions_from_wallet
    FOREIGN KEY (from_wallet_id)
    REFERENCES wallets(id)
    ON DELETE SET NULL,

    CONSTRAINT fk_transactions_to_wallet
    FOREIGN KEY (to_wallet_id)
    REFERENCES wallets(id)
    ON DELETE SET NULL,

    -- Проверки
    CONSTRAINT chk_amount_positive
    CHECK (amount > 0),

    CONSTRAINT chk_transfer_wallets
    CHECK (
(type != 'transfer') OR
(type = 'transfer' AND from_wallet_id IS NOT NULL AND to_wallet_id IS NOT NULL)
    )
    );

-- Индексы для производительности
CREATE INDEX idx_transactions_wallet_id ON transactions(wallet_id);
CREATE INDEX idx_transactions_created_at ON transactions(created_at DESC);
CREATE INDEX idx_transactions_from_wallet ON transactions(from_wallet_id) WHERE from_wallet_id IS NOT NULL;
CREATE INDEX idx_transactions_to_wallet ON transactions(to_wallet_id) WHERE to_wallet_id IS NOT NULL;

-- Уникальный индекс для идемпотентности
CREATE UNIQUE INDEX idx_transactions_idempotency
    ON transactions(idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- Комментарии к таблице
COMMENT ON TABLE transactions IS 'История всех финансовых операций';
COMMENT ON COLUMN transactions.amount IS 'Сумма в минимальных единицах валюты (копейки, центы)';
COMMENT ON COLUMN transactions.idempotency_key IS 'Ключ для предотвращения дублирования операций';
COMMENT ON COLUMN transactions.type IS 'Тип транзакции: deposit (пополнение), withdraw (снятие), transfer (перевод)';
