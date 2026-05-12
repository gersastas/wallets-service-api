package tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gersastas/wallets-service-api/internal/database"
	httpserver "github.com/gersastas/wallets-service-api/internal/transport/http/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServerWithTransactions(t *testing.T) (*http.Client, string, *sql.DB) {
	db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/wallet_db?sslmode=disable")
	require.NoError(t, err)

	err = db.Ping()
	require.NoError(t, err)

	err = database.RunMigrations(db)
	require.NoError(t, err)

	_, err = db.Exec("DELETE FROM transactions")
	require.NoError(t, err)

	_, err = db.Exec("DELETE FROM wallets")
	require.NoError(t, err)

	walletRepo := database.NewWalletRepository(db)
	transactionRepo := database.NewTransactionRepository(db)

	server := httpserver.New(":8080", walletRepo, transactionRepo, db)

	go func() {
		_ = server.Run()
	}()

	time.Sleep(100 * time.Millisecond)

	baseURL := "http://localhost:8080"
	client := &http.Client{Timeout: 5 * time.Second}

	return client, baseURL, db
}

func TestDeposit_Success(t *testing.T) {
	client, baseURL, _ := setupTestServerWithTransactions(t)

	walletID := createTestWallet(t, client, baseURL, "550e8400-e29b-41d4-a716-446655440000", "Test Wallet", "USD")

	depositReq := map[string]interface{}{
		"amount":          1000,
		"idempotency_key": "deposit-key-1",
	}
	body, _ := json.Marshal(depositReq)

	resp, err := client.Post(baseURL+"/wallets/"+walletID+"/deposit", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var txResp httpserver.TransactionResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&txResp))

	assert.Equal(t, "deposit", txResp.Type)
	assert.Equal(t, int64(1000), txResp.Amount)
	assert.Equal(t, "USD", txResp.Currency)
	assert.Equal(t, "deposit-key-1", txResp.IdempotencyKey)

	wallet := getWallet(t, client, baseURL, walletID)
	assert.Equal(t, int64(1000), wallet.Balance)
}

func TestDeposit_Idempotency(t *testing.T) {
	client, baseURL, _ := setupTestServerWithTransactions(t)

	walletID := createTestWallet(t, client, baseURL, "550e8400-e29b-41d4-a716-446655440000", "Test", "USD")

	depositReq := map[string]interface{}{
		"amount":          500,
		"idempotency_key": "same-key",
	}
	body, _ := json.Marshal(depositReq)

	resp1, _ := client.Post(baseURL+"/wallets/"+walletID+"/deposit", "application/json", bytes.NewReader(body))
	defer func() { _ = resp1.Body.Close() }()
	assert.Equal(t, http.StatusCreated, resp1.StatusCode)

	var tx1 httpserver.TransactionResponse
	require.NoError(t, json.NewDecoder(resp1.Body).Decode(&tx1))

	body, _ = json.Marshal(depositReq)
	resp2, _ := client.Post(baseURL+"/wallets/"+walletID+"/deposit", "application/json", bytes.NewReader(body))
	defer func() { _ = resp2.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	var tx2 httpserver.TransactionResponse
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&tx2))

	assert.Equal(t, tx1.ID, tx2.ID)

	wallet := getWallet(t, client, baseURL, walletID)
	assert.Equal(t, int64(500), wallet.Balance)
}

func TestWithdraw_Success(t *testing.T) {
	client, baseURL, _ := setupTestServerWithTransactions(t)

	walletID := createTestWallet(t, client, baseURL, "550e8400-e29b-41d4-a716-446655440000", "Test", "USD")

	deposit(t, client, baseURL, walletID, 1000, "deposit-1")

	withdrawReq := map[string]interface{}{
		"amount":          300,
		"idempotency_key": "withdraw-1",
	}
	body, _ := json.Marshal(withdrawReq)

	resp, _ := client.Post(baseURL+"/wallets/"+walletID+"/withdraw", "application/json", bytes.NewReader(body))
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var txResp httpserver.TransactionResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&txResp))

	assert.Equal(t, "withdraw", txResp.Type)
	assert.Equal(t, int64(300), txResp.Amount)

	wallet := getWallet(t, client, baseURL, walletID)
	assert.Equal(t, int64(700), wallet.Balance)
}

func TestWithdraw_InsufficientFunds(t *testing.T) {
	client, baseURL, _ := setupTestServerWithTransactions(t)

	walletID := createTestWallet(t, client, baseURL, "550e8400-e29b-41d4-a716-446655440000", "Test", "USD")

	deposit(t, client, baseURL, walletID, 100, "deposit-1")

	withdrawReq := map[string]interface{}{
		"amount":          500,
		"idempotency_key": "withdraw-fail",
	}
	body, _ := json.Marshal(withdrawReq)

	resp, _ := client.Post(baseURL+"/wallets/"+walletID+"/withdraw", "application/json", bytes.NewReader(body))
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpserver.ErrorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Contains(t, errResp.Error, "insufficient funds")

	wallet := getWallet(t, client, baseURL, walletID)
	assert.Equal(t, int64(100), wallet.Balance)
}

func TestTransfer_Success(t *testing.T) {
	client, baseURL, _ := setupTestServerWithTransactions(t)

	wallet1ID := createTestWallet(t, client, baseURL, "550e8400-e29b-41d4-a716-446655440000", "Wallet 1", "USD")
	wallet2ID := createTestWallet(t, client, baseURL, "550e8400-e29b-41d4-a716-446655440000", "Wallet 2", "USD")

	deposit(t, client, baseURL, wallet1ID, 1000, "deposit-1")

	transferReq := map[string]interface{}{
		"from_wallet_id":  wallet1ID,
		"to_wallet_id":    wallet2ID,
		"amount":          300,
		"idempotency_key": "transfer-1",
	}
	body, _ := json.Marshal(transferReq)

	resp, _ := client.Post(baseURL+"/wallets/transfer", "application/json", bytes.NewReader(body))
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	wallet1 := getWallet(t, client, baseURL, wallet1ID)
	wallet2 := getWallet(t, client, baseURL, wallet2ID)

	assert.Equal(t, int64(700), wallet1.Balance)
	assert.Equal(t, int64(300), wallet2.Balance)
}

func TestTransfer_InsufficientFunds(t *testing.T) {
	client, baseURL, _ := setupTestServerWithTransactions(t)

	wallet1ID := createTestWallet(t, client, baseURL, "550e8400-e29b-41d4-a716-446655440000", "Wallet 1", "USD")
	wallet2ID := createTestWallet(t, client, baseURL, "550e8400-e29b-41d4-a716-446655440000", "Wallet 2", "USD")

	deposit(t, client, baseURL, wallet1ID, 100, "deposit-1")

	transferReq := map[string]interface{}{
		"from_wallet_id":  wallet1ID,
		"to_wallet_id":    wallet2ID,
		"amount":          500,
		"idempotency_key": "transfer-fail",
	}
	body, _ := json.Marshal(transferReq)

	resp, _ := client.Post(baseURL+"/wallets/transfer", "application/json", bytes.NewReader(body))
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	wallet1 := getWallet(t, client, baseURL, wallet1ID)
	wallet2 := getWallet(t, client, baseURL, wallet2ID)

	assert.Equal(t, int64(100), wallet1.Balance)
	assert.Equal(t, int64(0), wallet2.Balance)
}

func TestListTransactions(t *testing.T) {
	client, baseURL, _ := setupTestServerWithTransactions(t)

	walletID := createTestWallet(t, client, baseURL, "550e8400-e29b-41d4-a716-446655440000", "Test", "USD")

	deposit(t, client, baseURL, walletID, 1000, "deposit-1")
	deposit(t, client, baseURL, walletID, 500, "deposit-2")

	withdrawReq := map[string]interface{}{
		"amount":          200,
		"idempotency_key": "withdraw-1",
	}
	body, _ := json.Marshal(withdrawReq)
	_, _ = client.Post(baseURL+"/wallets/"+walletID+"/withdraw", "application/json", bytes.NewReader(body))

	resp, _ := client.Get(baseURL + "/wallets/" + walletID + "/transactions")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var transactions []httpserver.TransactionResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&transactions))

	assert.Len(t, transactions, 3)

	assert.Equal(t, "withdraw", transactions[0].Type)
	assert.Equal(t, "deposit", transactions[1].Type)
	assert.Equal(t, "deposit", transactions[2].Type)
}

// ====== Helper функции ======

func createTestWallet(t *testing.T, client *http.Client, baseURL, userID, name, currency string) string {
	reqBody := map[string]string{
		"user_id":  userID,
		"name":     name,
		"currency": currency,
	}
	body, _ := json.Marshal(reqBody)

	resp, err := client.Post(baseURL+"/wallets", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var wallet httpserver.WalletResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wallet))
	return wallet.ID
}

func getWallet(t *testing.T, client *http.Client, baseURL, walletID string) httpserver.WalletResponse {
	resp, err := client.Get(baseURL + "/wallets/" + walletID)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var wallet httpserver.WalletResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wallet))
	return wallet
}

func deposit(t *testing.T, client *http.Client, baseURL, walletID string, amount int64, key string) {
	depositReq := map[string]interface{}{
		"amount":          amount,
		"idempotency_key": key,
	}
	body, _ := json.Marshal(depositReq)
	resp, _ := client.Post(baseURL+"/wallets/"+walletID+"/deposit", "application/json", bytes.NewReader(body))
	defer func() { _ = resp.Body.Close() }()
}
