package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gersastas/wallets-service-api/internal/auth"
	"github.com/gersastas/wallets-service-api/internal/database"
	"github.com/gersastas/wallets-service-api/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userIDKey contextKey = "userID"

type Server struct {
	httpServer      *http.Server
	walletRepo      *database.WalletRepository
	transactionRepo *database.TransactionRepository
	userRepo        *database.UserRepository
	db              *sql.DB
	jwtSecret       string
}

type CreateWalletRequest struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
}

func (r *CreateWalletRequest) Validate() error {
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

type DepositRequest struct {
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (r *DepositRequest) Validate() error {
	if r.Amount <= 0 {
		return errors.New("amount must be positive")
	}
	if r.IdempotencyKey == "" {
		return errors.New("idempotency_key is required")
	}
	return nil
}

type WithdrawRequest struct {
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (r *WithdrawRequest) Validate() error {
	if r.Amount <= 0 {
		return errors.New("amount must be positive")
	}
	if r.IdempotencyKey == "" {
		return errors.New("idempotency_key is required")
	}
	return nil
}

type TransferRequest struct {
	FromWalletID   string `json:"from_wallet_id"`
	ToWalletID     string `json:"to_wallet_id"`
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (r *TransferRequest) Validate() error {
	if r.FromWalletID == "" {
		return errors.New("from_wallet_id is required")
	}
	if r.ToWalletID == "" {
		return errors.New("to_wallet_id is required")
	}
	if r.FromWalletID == r.ToWalletID {
		return errors.New("cannot transfer to the same wallet")
	}
	if _, err := uuid.Parse(r.FromWalletID); err != nil {
		return errors.New("from_wallet_id must be valid UUID")
	}
	if _, err := uuid.Parse(r.ToWalletID); err != nil {
		return errors.New("to_wallet_id must be valid UUID")
	}
	if r.Amount <= 0 {
		return errors.New("amount must be positive")
	}
	if r.IdempotencyKey == "" {
		return errors.New("idempotency_key is required")
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

type TransactionResponse struct {
	ID             string    `json:"id"`
	WalletID       string    `json:"wallet_id"`
	Type           string    `json:"type"`
	Amount         int64     `json:"amount"`
	Currency       string    `json:"currency"`
	FromWalletID   *string   `json:"from_wallet_id,omitempty"`
	ToWalletID     *string   `json:"to_wallet_id,omitempty"`
	Description    string    `json:"description,omitempty"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func New(address string, walletRepo *database.WalletRepository, transactionRepo *database.TransactionRepository, userRepo *database.UserRepository, db *sql.DB, jwtSecret string) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	s := &Server{
		walletRepo:      walletRepo,
		transactionRepo: transactionRepo,
		userRepo:        userRepo,
		db:              db,
		jwtSecret:       jwtSecret,
	}

	r.Post("/auth/register", s.handleRegister)
	r.Post("/auth/login", s.handleLogin)
	r.Get("/health", s.handleHealth)

	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)

		r.Post("/wallets", s.handleCreateWallet)
		r.Get("/wallets/{id}", s.handleGetWallet)
		r.Put("/wallets/{id}", s.handleUpdateWallet)
		r.Delete("/wallets/{id}", s.handleDeleteWallet)
		r.Get("/wallets", s.handleListWallets)

		r.Post("/wallets/{id}/deposit", s.handleDeposit)
		r.Post("/wallets/{id}/withdraw", s.handleWithdraw)
		r.Post("/wallets/transfer", s.handleTransfer)
		r.Get("/wallets/{id}/transactions", s.handleListTransactions)
	})

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

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.db.PingContext(r.Context()); err != nil {
		logrus.WithError(err).Error("database health check failed")
		s.sendError(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	s.sendJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		s.sendError(w, "email is required", http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		s.sendError(w, "password is required", http.StatusBadRequest)
		return
	}

	existing, err := s.userRepo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		logrus.WithError(err).Error("failed to get user by email")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if existing != nil {
		s.sendError(w, "email already taken", http.StatusConflict)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logrus.WithError(err).Error("failed to hash password")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	user := &models.User{
		ID:           uuid.NewString(),
		Email:        req.Email,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.Create(r.Context(), user); err != nil {
		logrus.WithError(err).Error("failed to create user")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.sendJSON(w, map[string]string{
		"id":    user.ID,
		"email": user.Email,
	}, http.StatusCreated)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user, err := s.userRepo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		logrus.WithError(err).Error("failed to get user by email")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if user == nil {
		s.sendError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.sendError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.Generate(user.ID, s.jwtSecret)
	if err != nil {
		logrus.WithError(err).Error("failed to generate token")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.sendJSON(w, map[string]string{"token": token}, http.StatusOK)
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

	userID, ok := getUserID(r)
	if !ok {
		s.sendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	now := time.Now()
	wallet := &models.Wallet{
		ID:        uuid.NewString(),
		UserID:    userID,
		Name:      req.Name,
		Balance:   0,
		Currency:  req.Currency,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.walletRepo.Create(r.Context(), wallet); err != nil {
		logrus.WithError(err).Error("failed to create wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.sendJSON(w, walletToResponse(wallet), http.StatusCreated)
}

func (s *Server) handleGetWallet(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")
	if walletID == "" {
		s.sendError(w, "wallet_id is required", http.StatusBadRequest)
		return
	}

	wallet, err := s.walletRepo.GetByID(r.Context(), walletID)
	if err != nil {
		logrus.WithError(err).Error("failed to get wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if wallet == nil {
		s.sendError(w, "wallet not found", http.StatusNotFound)
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		s.sendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if wallet.UserID != userID {
		s.sendError(w, "forbidden", http.StatusForbidden)
		return
	}

	s.sendJSON(w, walletToResponse(wallet), http.StatusOK)
}

func (s *Server) handleUpdateWallet(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")
	if walletID == "" {
		s.sendError(w, "wallet_id is required", http.StatusBadRequest)
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

	wallet, err := s.walletRepo.GetByID(r.Context(), walletID)
	if err != nil {
		logrus.WithError(err).Error("failed to get wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if wallet == nil {
		s.sendError(w, "wallet not found", http.StatusNotFound)
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		s.sendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if wallet.UserID != userID {
		s.sendError(w, "forbidden", http.StatusForbidden)
		return
	}

	wallet.Name = req.Name
	wallet.UpdatedAt = time.Now()

	if err := s.walletRepo.Update(r.Context(), wallet); err != nil {
		logrus.WithError(err).Error("failed to update wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.sendJSON(w, walletToResponse(wallet), http.StatusOK)
}

func (s *Server) handleDeleteWallet(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")
	if walletID == "" {
		s.sendError(w, "wallet_id is required", http.StatusBadRequest)
		return
	}

	wallet, err := s.walletRepo.GetByID(r.Context(), walletID)
	if err != nil {
		logrus.WithError(err).Error("failed to get wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if wallet == nil {
		s.sendError(w, "wallet not found", http.StatusNotFound)
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		s.sendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if wallet.UserID != userID {
		s.sendError(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := s.walletRepo.Delete(r.Context(), walletID); err != nil {
		logrus.WithError(err).Error("failed to delete wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListWallets(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		s.sendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	wallets, err := s.walletRepo.List(r.Context(), userID, 10, 0)
	if err != nil {
		logrus.WithError(err).Error("failed to list wallets")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	response := make([]WalletResponse, 0, len(wallets))
	for _, wallet := range wallets {
		response = append(response, walletToResponse(wallet))
	}

	s.sendJSON(w, response, http.StatusOK)
}

func (s *Server) handleDeposit(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")
	if walletID == "" {
		s.sendError(w, "wallet_id is required", http.StatusBadRequest)
		return
	}

	var req DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	existingTx, err := s.transactionRepo.GetByIdempotencyKey(r.Context(), req.IdempotencyKey)
	if err != nil {
		logrus.WithError(err).Error("failed to check idempotency")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if existingTx != nil {
		s.sendJSON(w, transactionToResponse(existingTx), http.StatusOK)
		return
	}

	wallet, err := s.walletRepo.GetByID(r.Context(), walletID)
	if err != nil {
		logrus.WithError(err).Error("failed to get wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if wallet == nil {
		s.sendError(w, "wallet not found", http.StatusNotFound)
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		s.sendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if wallet.UserID != userID {
		s.sendError(w, "forbidden", http.StatusForbidden)
		return
	}

	now := time.Now()
	transaction := &models.Transaction{
		ID:             uuid.NewString(),
		WalletID:       walletID,
		Type:           models.TransactionTypeDeposit,
		Amount:         req.Amount,
		Currency:       wallet.Currency,
		Description:    "Deposit",
		IdempotencyKey: &req.IdempotencyKey,
		CreatedAt:      now,
	}

	wallet.Balance += req.Amount
	wallet.UpdatedAt = now

	if err := s.transactionRepo.Create(r.Context(), transaction); err != nil {
		logrus.WithError(err).Error("failed to create transaction")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := s.walletRepo.Update(r.Context(), wallet); err != nil {
		logrus.WithError(err).Error("failed to update wallet balance")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.sendJSON(w, transactionToResponse(transaction), http.StatusCreated)
}

func (s *Server) handleWithdraw(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")
	if walletID == "" {
		s.sendError(w, "wallet_id is required", http.StatusBadRequest)
		return
	}

	var req WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	existingTx, err := s.transactionRepo.GetByIdempotencyKey(r.Context(), req.IdempotencyKey)
	if err != nil {
		logrus.WithError(err).Error("failed to check idempotency")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if existingTx != nil {
		s.sendJSON(w, transactionToResponse(existingTx), http.StatusOK)
		return
	}

	wallet, err := s.walletRepo.GetByID(r.Context(), walletID)
	if err != nil {
		logrus.WithError(err).Error("failed to get wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if wallet == nil {
		s.sendError(w, "wallet not found", http.StatusNotFound)
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		s.sendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if wallet.UserID != userID {
		s.sendError(w, "forbidden", http.StatusForbidden)
		return
	}

	if wallet.Balance < req.Amount {
		s.sendError(w, "insufficient funds", http.StatusBadRequest)
		return
	}

	now := time.Now()
	transaction := &models.Transaction{
		ID:             uuid.NewString(),
		WalletID:       walletID,
		Type:           models.TransactionTypeWithdraw,
		Amount:         req.Amount,
		Currency:       wallet.Currency,
		Description:    "Withdrawal",
		IdempotencyKey: &req.IdempotencyKey,
		CreatedAt:      now,
	}

	wallet.Balance -= req.Amount
	wallet.UpdatedAt = now

	if err := s.transactionRepo.Create(r.Context(), transaction); err != nil {
		logrus.WithError(err).Error("failed to create transaction")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := s.walletRepo.Update(r.Context(), wallet); err != nil {
		logrus.WithError(err).Error("failed to update wallet balance")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.sendJSON(w, transactionToResponse(transaction), http.StatusCreated)
}

func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	existingTx, err := s.transactionRepo.GetByIdempotencyKey(r.Context(), req.IdempotencyKey)
	if err != nil {
		logrus.WithError(err).Error("failed to check idempotency")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if existingTx != nil {
		s.sendJSON(w, transactionToResponse(existingTx), http.StatusOK)
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		s.sendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	dbTx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		logrus.WithError(err).Error("failed to begin transaction")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	defer func() {
		if rollbackErr := dbTx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			logrus.WithError(rollbackErr).Error("failed to rollback transaction")
		}
	}()

	fromWallet, err := s.getWalletForUpdate(r.Context(), dbTx, req.FromWalletID)
	if err != nil {
		logrus.WithError(err).Error("failed to get from_wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if fromWallet == nil {
		s.sendError(w, "from_wallet not found", http.StatusNotFound)
		return
	}

	if fromWallet.UserID != userID {
		s.sendError(w, "forbidden", http.StatusForbidden)
		return
	}

	toWallet, err := s.getWalletForUpdate(r.Context(), dbTx, req.ToWalletID)
	if err != nil {
		logrus.WithError(err).Error("failed to get to_wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if toWallet == nil {
		s.sendError(w, "to_wallet not found", http.StatusNotFound)
		return
	}

	if fromWallet.Currency != toWallet.Currency {
		s.sendError(w, "currency mismatch", http.StatusBadRequest)
		return
	}

	if fromWallet.Balance < req.Amount {
		s.sendError(w, "insufficient funds", http.StatusBadRequest)
		return
	}

	now := time.Now()
	fromWallet.Balance -= req.Amount
	fromWallet.UpdatedAt = now
	toWallet.Balance += req.Amount
	toWallet.UpdatedAt = now

	if err := s.updateWalletInTx(r.Context(), dbTx, fromWallet); err != nil {
		logrus.WithError(err).Error("failed to update from_wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := s.updateWalletInTx(r.Context(), dbTx, toWallet); err != nil {
		logrus.WithError(err).Error("failed to update to_wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	withdrawTx := &models.Transaction{
		ID:             uuid.NewString(),
		WalletID:       req.FromWalletID,
		Type:           models.TransactionTypeTransfer,
		Amount:         req.Amount,
		Currency:       fromWallet.Currency,
		FromWalletID:   &req.FromWalletID,
		ToWalletID:     &req.ToWalletID,
		Description:    fmt.Sprintf("Transfer to %s", req.ToWalletID),
		IdempotencyKey: &req.IdempotencyKey,
		CreatedAt:      now,
	}

	depositTx := &models.Transaction{
		ID:           uuid.NewString(),
		WalletID:     req.ToWalletID,
		Type:         models.TransactionTypeTransfer,
		Amount:       req.Amount,
		Currency:     toWallet.Currency,
		FromWalletID: &req.FromWalletID,
		ToWalletID:   &req.ToWalletID,
		Description:  fmt.Sprintf("Transfer from %s", req.FromWalletID),
		CreatedAt:    now,
	}

	if err := s.transactionRepo.CreateWithTx(r.Context(), dbTx, withdrawTx); err != nil {
		logrus.WithError(err).Error("failed to create withdraw transaction")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := s.transactionRepo.CreateWithTx(r.Context(), dbTx, depositTx); err != nil {
		logrus.WithError(err).Error("failed to create deposit transaction")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := dbTx.Commit(); err != nil {
		logrus.WithError(err).Error("failed to commit transaction")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.sendJSON(w, transactionToResponse(withdrawTx), http.StatusCreated)
}

func (s *Server) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")
	if walletID == "" {
		s.sendError(w, "wallet_id is required", http.StatusBadRequest)
		return
	}

	wallet, err := s.walletRepo.GetByID(r.Context(), walletID)
	if err != nil {
		logrus.WithError(err).Error("failed to get wallet")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if wallet == nil {
		s.sendError(w, "wallet not found", http.StatusNotFound)
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		s.sendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if wallet.UserID != userID {
		s.sendError(w, "forbidden", http.StatusForbidden)
		return
	}

	transactions, err := s.transactionRepo.ListByWallet(r.Context(), walletID, 10, 0)
	if err != nil {
		logrus.WithError(err).Error("failed to list transactions")
		s.sendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	response := make([]TransactionResponse, 0, len(transactions))
	for _, tx := range transactions {
		response = append(response, transactionToResponse(tx))
	}

	s.sendJSON(w, response, http.StatusOK)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.sendError(w, "authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			s.sendError(w, "invalid authorization header format", http.StatusUnauthorized)
			return
		}

		claims, err := auth.Validate(parts[1], s.jwtSecret)
		if err != nil {
			s.sendError(w, "invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) getWalletForUpdate(ctx context.Context, tx *sql.Tx, walletID string) (*models.Wallet, error) {
	query := `
		SELECT id, user_id, name, balance, currency, created_at, updated_at, deleted_at
		FROM wallets
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`

	wallet := &models.Wallet{}
	err := tx.QueryRowContext(ctx, query, walletID).Scan(
		&wallet.ID, &wallet.UserID, &wallet.Name, &wallet.Balance,
		&wallet.Currency, &wallet.CreatedAt, &wallet.UpdatedAt, &wallet.DeletedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return wallet, nil
}

func (s *Server) updateWalletInTx(ctx context.Context, tx *sql.Tx, wallet *models.Wallet) error {
	query := `
		UPDATE wallets
		SET name = $1, balance = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := tx.ExecContext(ctx, query, wallet.Name, wallet.Balance, wallet.UpdatedAt, wallet.ID)
	return err
}

func getUserID(r *http.Request) (string, bool) {
	userID, ok := r.Context().Value(userIDKey).(string)
	return userID, ok
}

func walletToResponse(wallet *models.Wallet) WalletResponse {
	return WalletResponse{
		ID:        wallet.ID,
		UserID:    wallet.UserID,
		Name:      wallet.Name,
		Balance:   wallet.Balance,
		Currency:  wallet.Currency,
		CreatedAt: wallet.CreatedAt,
		UpdatedAt: wallet.UpdatedAt,
	}
}

func transactionToResponse(tx *models.Transaction) TransactionResponse {
	resp := TransactionResponse{
		ID:          tx.ID,
		WalletID:    tx.WalletID,
		Type:        string(tx.Type),
		Amount:      tx.Amount,
		Currency:    tx.Currency,
		Description: tx.Description,
		CreatedAt:   tx.CreatedAt,
	}

	if tx.FromWalletID != nil {
		resp.FromWalletID = tx.FromWalletID
	}

	if tx.ToWalletID != nil {
		resp.ToWalletID = tx.ToWalletID
	}

	if tx.IdempotencyKey != nil {
		resp.IdempotencyKey = *tx.IdempotencyKey
	}

	return resp
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