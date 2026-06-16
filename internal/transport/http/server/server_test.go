package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gersastas/wallets-service-api/internal/config"
	"github.com/gersastas/wallets-service-api/internal/database"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestServer_Integration(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/wallet_db?sslmode=disable")
	require.NoError(t, err)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("failed to close database: %v", closeErr)
		}
	}()

	require.NoError(t, db.Ping())
	require.NoError(t, database.RunMigrations(db))

	_, err = db.ExecContext(context.Background(), "DELETE FROM transactions")
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), "DELETE FROM wallets")
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), "DELETE FROM users")
	require.NoError(t, err)

	originalAddr := os.Getenv("HTTP_BIND_ADDR")
	defer func() {
		if originalAddr != "" {
			require.NoError(t, os.Setenv("HTTP_BIND_ADDR", originalAddr))
		} else {
			require.NoError(t, os.Unsetenv("HTTP_BIND_ADDR"))
		}
	}()

	port, err := getFreePort()
	require.NoError(t, err)

	testAddr := "localhost:" + port
	require.NoError(t, os.Setenv("HTTP_BIND_ADDR", testAddr))

	cfg := config.New()

	srv := New(
		cfg.GetHTTPBindAddr(),
		database.NewWalletRepository(db),
		database.NewTransactionRepository(db),
		database.NewUserRepository(db),
		db,
		cfg.GetJWTSecret(),
	)

	ready := make(chan struct{})
	go func() {
		close(ready)
		_ = srv.Run()
	}()

	select {
	case <-ready:
		time.Sleep(100 * time.Millisecond)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not start in time")
	}

	baseURL := "http://" + testAddr
	client := &http.Client{Timeout: 2 * time.Second}

	// Регистрация
	regBody, _ := json.Marshal(map[string]string{"email": "test@test.com", "password": "secret123"})
	regResp, err := client.Post(baseURL+"/auth/register", "application/json", bytes.NewReader(regBody))
	require.NoError(t, err)
	defer func() { _ = regResp.Body.Close() }()
	require.Equal(t, http.StatusCreated, regResp.StatusCode)

	// Логин
	loginBody, _ := json.Marshal(map[string]string{"email": "test@test.com", "password": "secret123"})
	loginResp, err := client.Post(baseURL+"/auth/login", "application/json", bytes.NewReader(loginBody))
	require.NoError(t, err)
	defer func() { _ = loginResp.Body.Close() }()
	require.Equal(t, http.StatusOK, loginResp.StatusCode)

	var loginResult map[string]string
	require.NoError(t, json.NewDecoder(loginResp.Body).Decode(&loginResult))
	token := loginResult["token"]
	require.NotEmpty(t, token)

	// 100 параллельных запросов с токеном
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req, reqErr := http.NewRequest(http.MethodGet, baseURL+"/wallets", nil)
			require.NoError(t, reqErr)
			req.Header.Set("Authorization", "Bearer "+token)

			resp, respErr := client.Do(req)
			require.NoError(t, respErr)
			defer func() {
				if closeErr := resp.Body.Close(); closeErr != nil {
					t.Logf("failed to close response body: %v", closeErr)
				}
			}()

			require.Equal(t, http.StatusOK, resp.StatusCode)
		}()
	}

	wg.Wait()
	t.Log("all 100 requests completed successfully")
}

func getFreePort() (string, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := l.Close(); closeErr != nil {
			_ = closeErr
		}
	}()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return "", err
	}
	return port, nil
}
