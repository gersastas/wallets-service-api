package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	httpserver "github.com/gersastas/wallets-service-api/internal/transport/http/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer() *httptest.Server {
	server := httpserver.New(":8080")
	return httptest.NewServer(server.Handler())
}

func TestCreateWallet_Success(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	reqBody := map[string]string{
		"user_id":  "550e8400-e29b-41d4-a716-446655440000",
		"name":     "Test Wallet",
		"currency": "USD",
	}

	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(ts.URL+"/wallets", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result httpserver.WalletResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", result.UserID)
	assert.Equal(t, "Test Wallet", result.Name)
	assert.Equal(t, int64(0), result.Balance)
	assert.Equal(t, "USD", result.Currency)
}

func TestCreateWallet_ValidationErrors(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	testCases := []struct {
		name       string
		body       map[string]string
		wantStatus int
		wantError  string
	}{
		{
			name:       "empty user_id",
			body:       map[string]string{"name": "Test", "currency": "USD"},
			wantStatus: http.StatusBadRequest,
			wantError:  "user_id is required",
		},
		{
			name:       "invalid user_id",
			body:       map[string]string{"user_id": "invalid", "name": "Test", "currency": "USD"},
			wantStatus: http.StatusBadRequest,
			wantError:  "user_id must be valid UUID",
		},
		{
			name:       "empty name",
			body:       map[string]string{"user_id": "550e8400-e29b-41d4-a716-446655440000", "currency": "USD"},
			wantStatus: http.StatusBadRequest,
			wantError:  "name is required",
		},
		{
			name:       "empty currency",
			body:       map[string]string{"user_id": "550e8400-e29b-41d4-a716-446655440000", "name": "Test"},
			wantStatus: http.StatusBadRequest,
			wantError:  "currency is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			resp, err := http.Post(ts.URL+"/wallets", "application/json", bytes.NewReader(body))
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)

			var errResp httpserver.ErrorResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
			assert.Contains(t, errResp.Error, tc.wantError)
		})
	}
}

func TestGetWallet_Success(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	createBody := map[string]string{
		"user_id":  "550e8400-e29b-41d4-a716-446655440000",
		"name":     "Test Wallet",
		"currency": "USD",
	}
	body, _ := json.Marshal(createBody)
	createResp, _ := http.Post(ts.URL+"/wallets", "application/json", bytes.NewReader(body))
	defer func() { _ = createResp.Body.Close() }()

	var created httpserver.WalletResponse
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))

	getResp, err := http.Get(ts.URL + "/wallets/" + created.ID)
	require.NoError(t, err)
	defer func() { _ = getResp.Body.Close() }()

	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var result httpserver.WalletResponse
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&result))

	assert.Equal(t, created.ID, result.ID)
	assert.Equal(t, created.Name, result.Name)
}

func TestGetWallet_NotFound(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/wallets/550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUpdateWallet_Success(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	createBody := map[string]string{
		"user_id":  "550e8400-e29b-41d4-a716-446655440000",
		"name":     "Old Name",
		"currency": "USD",
	}
	body, _ := json.Marshal(createBody)
	createResp, _ := http.Post(ts.URL+"/wallets", "application/json", bytes.NewReader(body))
	defer func() { _ = createResp.Body.Close() }()

	var created httpserver.WalletResponse
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))

	updateBody := map[string]string{"name": "New Name"}
	body, _ = json.Marshal(updateBody)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/wallets/"+created.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	updateResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = updateResp.Body.Close() }()

	assert.Equal(t, http.StatusOK, updateResp.StatusCode)

	var updated httpserver.WalletResponse
	require.NoError(t, json.NewDecoder(updateResp.Body).Decode(&updated))

	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, created.ID, updated.ID)
}

func TestDeleteWallet_Success(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	// Создаём
	createBody := map[string]string{
		"user_id":  "550e8400-e29b-41d4-a716-446655440000",
		"name":     "To Delete",
		"currency": "USD",
	}
	body, _ := json.Marshal(createBody)
	createResp, _ := http.Post(ts.URL+"/wallets", "application/json", bytes.NewReader(body))
	defer func() { _ = createResp.Body.Close() }()

	var created httpserver.WalletResponse
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/wallets/"+created.ID, nil)
	deleteResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = deleteResp.Body.Close() }()

	assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode)

	getResp, _ := http.Get(ts.URL + "/wallets/" + created.ID)
	defer func() { _ = getResp.Body.Close() }()
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
}

func TestListWallets_Success(t *testing.T) {
	ts := setupTestServer()
	defer ts.Close()

	userID := "550e8400-e29b-41d4-a716-446655440000"

	for i := 1; i <= 2; i++ {
		createBody := map[string]interface{}{
			"user_id":  userID,
			"name":     "Wallet",
			"currency": "USD",
		}
		body, _ := json.Marshal(createBody)
		resp, _ := http.Post(ts.URL+"/wallets", "application/json", bytes.NewReader(body))
		_ = resp.Body.Close()
	}

	listResp, err := http.Get(ts.URL + "/wallets?user_id=" + userID)
	require.NoError(t, err)
	defer func() { _ = listResp.Body.Close() }()

	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	var wallets []httpserver.WalletResponse
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&wallets))

	assert.GreaterOrEqual(t, len(wallets), 2)
}
