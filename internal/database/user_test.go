package database

import (
	"context"
	"testing"
	"time"

	"github.com/gersastas/wallets-service-api/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)

	user := &models.User{
		ID:           uuid.NewString(),
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := repo.Create(context.Background(), user)
	require.NoError(t, err)
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)

	user := &models.User{
		ID:           uuid.NewString(),
		Email:        "find@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, repo.Create(context.Background(), user))

	got, err := repo.GetByEmail(context.Background(), "find@example.com")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, user.Email, got.Email)
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)

	got, err := repo.GetByEmail(context.Background(), "notfound@example.com")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestUserRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)

	user := &models.User{
		ID:           uuid.NewString(),
		Email:        "byid@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, repo.Create(context.Background(), user))

	got, err := repo.GetByID(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, user.ID, got.ID)
}