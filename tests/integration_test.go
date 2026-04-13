package integration_test

import (
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gersastas/wallets-service-api/internal/config"
	httpserver "github.com/gersastas/wallets-service-api/internal/transport/http/server"
	"github.com/stretchr/testify/assert"
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

	serverErr := make(chan error, 1)
	serverReady := make(chan struct{})

	// Запускаем сервер
	go func() {
		close(serverReady)
		if err := server.Run(); err != nil {
			serverErr <- err
		}
	}()

	select {
	case <-serverReady:
		time.Sleep(100 * time.Millisecond)
	case err := <-serverErr:
		t.Fatalf("server failed to start: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not start in time")
	}

	select {
	case err := <-serverErr:
		t.Fatalf("server crashed: %v", err)
	default:
	}

	client := &http.Client{Timeout: 2 * time.Second}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			resp, err := client.Get("http://" + testAddr + "/time")
			if err != nil {
				errors <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errors <- assert.AnError
			}
		}()
	}

	wg.Wait()
	close(errors)

	var errCount int
	for err := range errors {
		t.Errorf("request failed: %v", err)
		errCount++
	}

	if errCount > 0 {
		t.Fatalf("failed requests: %d/100", errCount)
	}

	t.Log("all 100 requests completed successfully")
}

func getFreePort() (string, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", err
	}
	defer l.Close()

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return "", err
	}

	return port, nil
}