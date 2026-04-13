package models

import (
	"time"

	"github.com/google/uuid"
)

type Wallet struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Balance   int64
	Currency  string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}
