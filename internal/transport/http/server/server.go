package server

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Server struct {
	httpServer *http.Server
	stats      map[string]int
	mu         sync.Mutex
}

func New(address string) *Server {
	s := &Server{
		stats: make(map[string]int),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/time", s.timeHandler)
	mux.HandleFunc("/stats", s.statsHandler)

	s.httpServer = &http.Server{
		Addr:    address,
		Handler: mux,
	}

	return s
}

func (s *Server) Run() error {
	err := s.httpServer.ListenAndServe()

	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func (s *Server) timeHandler(w http.ResponseWriter, r *http.Request) {
	now := time.Now().Format(time.RFC3339)
	ip := getIP(r)

	s.mu.Lock()
	s.stats[ip]++
	s.mu.Unlock()

	if _, err := w.Write([]byte(now)); err != nil {
		logrus.WithFields(logrus.Fields{
			"error":       err,
			"remote_addr": r.RemoteAddr,
			"path":        r.URL.Path,
		}).Warn("failed to write response")
	}
}

func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for ip, count := range s.stats {
		if _, err := w.Write([]byte(ip + "\t" + strconv.Itoa(count) + "\n")); err != nil {
			logrus.Warn("failed to write response", err)
		}
	}
}

func getIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return host
}
