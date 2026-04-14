package tests

import (
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gersastas/wallets-service-api/internal/config"
	httpserver "github.com/gersastas/wallets-service-api/internal/transport/http/server"
	"github.com/stretchr/testify/require"
)

func TestServer_Integration(t *testing.T) {
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
	require.Equal(t, testAddr, cfg.GetHTTPBindAddr())

	server := httpserver.New(cfg.GetHTTPBindAddr())

	ready := make(chan struct{})
	go func() {
		close(ready)
		_ = server.Run()
	}()

	select {
	case <-ready:
		time.Sleep(100 * time.Millisecond)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not start in time")
	}

	client := &http.Client{Timeout: 2 * time.Second}
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			resp, err := client.Get("http://" + testAddr + "/wallets?user_id=550e8400-e29b-41d4-a716-446655440000")
			require.NoError(t, err)

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
