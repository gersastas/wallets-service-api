package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gersastas/wallets-service-api/internal/database"
	"github.com/gersastas/wallets-service-api/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type Server struct {
	httpServer *http.Server
	repo       *database.WalletRepository
}

func New(address string, repo *database.WalletRepository) *Server {
	r := chi.NewRouter()

	s := &Server{
		repo: repo,
	}

	r.Post("/wallets", s.handleCreateWallet)
	r.Get("/wallets/{id}", s.handleGetWallet)
	r.Put("/wallets/{id}", s.handleUpdateWallet)
	r.Delete("/wallets/{id}", s.handleDeleteWallet)
	r.Get("/wallets", s.handleListWallets)

	s.httpServer = &http.Server{
		Addr:    address,
		Handler: r,
	}

	return s
}

func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

func (s *Server) Run() error {
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

type CreateWalletRequest struct {
	UserID   string `json:"user_id"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
}

func (r *CreateWalletRequest) Validate() error {
	if r.UserID == "" {
		return errors.New("user_id is required")
	}
	if _, err := uuid.Parse(r.UserID); err != nil {
		return errors.New("user_id must be valid UUID")
	}
	if r.Name == "" {
		return errors.New("name is required")
	}
	if r.Currency == "" {
		return errors.New("currency is required")
	}
	return nil
}

type UpdateWalletRequest struct {
	Name string `json:"name"`
}

func (r *UpdateWalletRequest) Validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

type WalletResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Balance   int64     `json:"balance"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (s *Server) handleCreateWallet(w http.ResponseWriter, r *http.Request) {
	var req CreateWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	now := time.Now()
	walletID := uuid.New()
	userID, _ := uuid.Parse(req.UserID)

	wallet := &models.Wallet{
		ID:        walletID,
		UserID:    userID,
		Name:      req.Name,
		Balance:   0,
		Currency:  req.Currency,
		CreatedAt: now,
		UpdatedAt: now,
		DeletedAt: nil,
	}

	if err := s.repo.Create(r.Context(), wallet); err != nil {
		logrus.WithError(err).Error("failed to create wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := WalletResponse{
		ID:        wallet.ID.String(),
		UserID:    wallet.UserID.String(),
		Name:      wallet.Name,
		Balance:   wallet.Balance,
		Currency:  wallet.Currency,
		CreatedAt: wallet.CreatedAt,
		UpdatedAt: wallet.UpdatedAt,
	}

	s.sendJSON(w, resp, http.StatusCreated)
}

func (s *Server) handleGetWallet(w http.ResponseWriter, r *http.Request) {
	walletIDStr := chi.URLParam(r, "id")
	if walletIDStr == "" {
		s.sendError(w, "wallet_id is required", http.StatusBadRequest)
		return
	}

	walletID, err := uuid.Parse(walletIDStr)
	if err != nil {
		s.sendError(w, "invalid wallet_id", http.StatusBadRequest)
		return
	}

	wallet, err := s.repo.GetByID(r.Context(), walletID)
	if err != nil {
		logrus.WithError(err).Error("failed to get wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if wallet == nil {
		s.sendError(w, "wallet not found", http.StatusNotFound)
		return
	}

	resp := WalletResponse{
		ID:        wallet.ID.String(),
		UserID:    wallet.UserID.String(),
		Name:      wallet.Name,
		Balance:   wallet.Balance,
		Currency:  wallet.Currency,
		CreatedAt: wallet.CreatedAt,
		UpdatedAt: wallet.UpdatedAt,
	}

	s.sendJSON(w, resp, http.StatusOK)
}

func (s *Server) handleUpdateWallet(w http.ResponseWriter, r *http.Request) {
	walletIDStr := chi.URLParam(r, "id")
	if walletIDStr == "" {
		s.sendError(w, "wallet_id is required", http.StatusBadRequest)
		return
	}

	walletID, err := uuid.Parse(walletIDStr)
	if err != nil {
		s.sendError(w, "invalid wallet_id", http.StatusBadRequest)
		return
	}

	var req UpdateWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	wallet, err := s.repo.GetByID(r.Context(), walletID)
	if err != nil {
		logrus.WithError(err).Error("failed to get wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if wallet == nil {
		s.sendError(w, "wallet not found", http.StatusNotFound)
		return
	}

	wallet.Name = req.Name
	wallet.UpdatedAt = time.Now()

	if err := s.repo.Update(r.Context(), wallet); err != nil {
		logrus.WithError(err).Error("failed to update wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := WalletResponse{
		ID:        wallet.ID.String(),
		UserID:    wallet.UserID.String(),
		Name:      wallet.Name,
		Balance:   wallet.Balance,
		Currency:  wallet.Currency,
		CreatedAt: wallet.CreatedAt,
		UpdatedAt: wallet.UpdatedAt,
	}

	s.sendJSON(w, resp, http.StatusOK)
}

func (s *Server) handleDeleteWallet(w http.ResponseWriter, r *http.Request) {
	walletIDStr := chi.URLParam(r, "id")
	if walletIDStr == "" {
		s.sendError(w, "wallet_id is required", http.StatusBadRequest)
		return
	}

	walletID, err := uuid.Parse(walletIDStr)
	if err != nil {
		s.sendError(w, "invalid wallet_id", http.StatusBadRequest)
		return
	}

	if err := s.repo.Delete(r.Context(), walletID); err != nil {
		logrus.WithError(err).Error("failed to delete wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListWallets(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		s.sendError(w, "user_id query parameter is required", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.sendError(w, "invalid user_id", http.StatusBadRequest)
		return
	}

	limit := 10
	offset := 0

	wallets, err := s.repo.List(r.Context(), userID, limit, offset)
	if err != nil {
		logrus.WithError(err).Error("failed to list wallets")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var response []WalletResponse
	for _, wallet := range wallets {
		response = append(response, WalletResponse{
			ID:        wallet.ID.String(),
			UserID:    wallet.UserID.String(),
			Name:      wallet.Name,
			Balance:   wallet.Balance,
			Currency:  wallet.Currency,
			CreatedAt: wallet.CreatedAt,
			UpdatedAt: wallet.UpdatedAt,
		})
	}

	s.sendJSON(w, response, http.StatusOK)
}

func (s *Server) sendJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logrus.WithError(err).Error("failed to encode response")
	}
}

func (s *Server) sendError(w http.ResponseWriter, message string, status int) {
	s.sendJSON(w, ErrorResponse{Error: message}, status)
}
