package models

import (
	"time"

	"github.com/google/uuid"
)
type TransactionType string

const (
	TransactionTypeDeposit  TransactionType = "deposit"
	TransactionTypeWithdraw TransactionType = "withdraw"
	TransactionTypeTransfer TransactionType = "transfer"
)
type Transaction struct {
	ID             uuid.UUID       `json:"id"`
	WalletID       uuid.UUID       `json:"wallet_id"`
	Type           TransactionType `json:"type"`
	Amount         int64           `json:"amount"`
	Currency       string          `json:"currency"`
	FromWalletID   *uuid.UUID      `json:"from_wallet_id,omitempty"`
	ToWalletID     *uuid.UUID      `json:"to_wallet_id,omitempty"`
	Description    string          `json:"description,omitempty"`
	IdempotencyKey *string         `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}
