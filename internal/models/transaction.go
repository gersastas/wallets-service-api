package models

import "time"

type TransactionType string

const (
	TransactionTypeDeposit  TransactionType = "deposit"
	TransactionTypeWithdraw TransactionType = "withdraw"
	TransactionTypeTransfer TransactionType = "transfer"
)

type Transaction struct {
	ID             string          `json:"id"`
	WalletID       string          `json:"wallet_id"`
	Type           TransactionType `json:"type"`
	Amount         int64           `json:"amount"`
	Currency       string          `json:"currency"`
	FromWalletID   *string         `json:"from_wallet_id,omitempty"`
	ToWalletID     *string         `json:"to_wallet_id,omitempty"`
	Description    string          `json:"description,omitempty"`
	IdempotencyKey *string         `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}
