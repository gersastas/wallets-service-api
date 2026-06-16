package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/gersastas/wallets-service-api/internal/models"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/wallet_db?sslmode=disable")
	require.NoError(t, err)

	err = db.Ping()
	require.NoError(t, err)

	err = RunMigrations(db)
	require.NoError(t, err)

	_, err = db.Exec("DELETE FROM transactions")
	require.NoError(t, err)

	_, err = db.Exec("DELETE FROM wallets")
	require.NoError(t, err)

	_, err = db.Exec("DELETE FROM users")
	require.NoError(t, err)

	return db
}

func TestWalletRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWalletRepository(db)

	wallet := &models.Wallet{
		ID:        uuid.NewString(),
		UserID:    uuid.NewString(),
		Name:      "Test Wallet",
		Balance:   0,
		Currency:  "USD",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.Create(context.Background(), wallet)
	require.NoError(t, err)

	got, err := repo.GetByID(context.Background(), wallet.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, wallet.ID, got.ID)
	assert.Equal(t, wallet.Name, got.Name)
	assert.Equal(t, wallet.Currency, got.Currency)
}

func TestWalletRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWalletRepository(db)

	got, err := repo.GetByID(context.Background(), uuid.NewString())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestWalletRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWalletRepository(db)

	wallet := &models.Wallet{
		ID:        uuid.NewString(),
		UserID:    uuid.NewString(),
		Name:      "Old Name",
		Balance:   0,
		Currency:  "USD",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := repo.Create(context.Background(), wallet)
	require.NoError(t, err)

	wallet.Name = "New Name"
	wallet.Balance = 500
	wallet.UpdatedAt = time.Now()

	err = repo.Update(context.Background(), wallet)
	require.NoError(t, err)

	got, err := repo.GetByID(context.Background(), wallet.ID)
	require.NoError(t, err)

	assert.Equal(t, "New Name", got.Name)
	assert.Equal(t, int64(500), got.Balance)
}

func TestWalletRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWalletRepository(db)

	wallet := &models.Wallet{
		ID:        uuid.NewString(),
		UserID:    uuid.NewString(),
		Name:      "To Delete",
		Balance:   0,
		Currency:  "USD",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := repo.Create(context.Background(), wallet)
	require.NoError(t, err)

	err = repo.Delete(context.Background(), wallet.ID)
	require.NoError(t, err)

	got, err := repo.GetByID(context.Background(), wallet.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestWalletRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWalletRepository(db)

	userID := uuid.NewString()

	for i := 0; i < 3; i++ {
		wallet := &models.Wallet{
			ID:        uuid.NewString(),
			UserID:    userID,
			Name:      "Wallet",
			Balance:   0,
			Currency:  "USD",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := repo.Create(context.Background(), wallet)
		require.NoError(t, err)
	}

	wallets, err := repo.List(context.Background(), userID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, wallets, 3)
}
