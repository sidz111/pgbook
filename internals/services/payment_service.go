package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"github.com/sidz111/pgbook/internals/repositories"
)

type PaymentService interface {
	CreatePayment(ctx context.Context, payment *models.Payment) error
	GetPaymentByID(ctx context.Context, id uuid.UUID) (*models.Payment, error)
	GetPaymentsByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.Payment, error)
	GetPaymentsByPG(ctx context.Context, pgID uuid.UUID) ([]models.Payment, error)

	// Payment Management
	UpdatePaymentStatus(ctx context.Context, paymentID uuid.UUID, status string) error
	VerifyPayment(ctx context.Context, paymentID uuid.UUID, adminRemarks string) error
	RejectPayment(ctx context.Context, paymentID uuid.UUID, reason string) error

	// Analytics
	GetPendingPayments(ctx context.Context, pgID uuid.UUID) ([]models.Payment, error)
	GetPaymentStats(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error)
	GetTenantPaymentHistory(ctx context.Context, tenantID uuid.UUID, months int) ([]models.Payment, error)
	GetMonthlyCollectionStats(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error)
	GetOutstandingPayments(ctx context.Context, pgID uuid.UUID) ([]models.Payment, error)

	// Monthly Rent
	GetMonthlyOutstanding(ctx context.Context, tenantID uuid.UUID, month string) (*models.Payment, error)
	InitiateMonthlyPayment(ctx context.Context, tenantID uuid.UUID, month string, amount float64) error
}

type paymentService struct {
	paymentRepo repositories.PaymentRepository
	tenantRepo  repositories.TenantRepository
	pgRepo      repositories.PGRepository
	roomRepo    repositories.RoomRepository
	logger      *slog.Logger
}

func NewPaymentService(
	paymentRepo repositories.PaymentRepository,
	tenantRepo repositories.TenantRepository,
	pgRepo repositories.PGRepository,
	roomRepo repositories.RoomRepository,
) PaymentService {
	return &paymentService{
		paymentRepo: paymentRepo,
		tenantRepo:  tenantRepo,
		pgRepo:      pgRepo,
		roomRepo:    roomRepo,
		logger:      slog.Default(),
	}
}

// CreatePayment creates a new payment record
func (s *paymentService) CreatePayment(ctx context.Context, payment *models.Payment) error {
	// Validation
	if payment.TenantID == uuid.Nil {
		return errors.New("tenant_id is required")
	}
	if payment.PGID == uuid.Nil {
		return errors.New("PG_id is required")
	}
	if payment.Amount <= 0 {
		return errors.New("payment amount must be greater than 0")
	}
	if payment.Method == "" {
		return errors.New("payment method is required")
	}
	if payment.ForMonth == "" {
		return errors.New("payment month is required")
	}

	// Verify tenant exists
	_, err := s.tenantRepo.GetTenantByID(ctx, payment.TenantID)
	if err != nil {
		return errors.New("tenant not found")
	}

	// Verify PG exists
	_, err = s.pgRepo.GetPGByID(ctx, payment.PGID)
	if err != nil {
		return errors.New("PG not found")
	}

	payment.ID = uuid.New()
	payment.Status = "pending"
	if payment.PaymentDate.IsZero() {
		payment.PaymentDate = time.Now()
	}

	if err := s.paymentRepo.CreatePayment(ctx, payment); err != nil {
		s.logger.Error("Failed to create payment", "error", err, "tenant_id", payment.TenantID)
		return errors.New("failed to create payment")
	}

	s.logger.Info("Payment created", "payment_id", payment.ID, "tenant_id", payment.TenantID, "amount", payment.Amount)
	return nil
}

// GetPaymentByID retrieves payment details
func (s *paymentService) GetPaymentByID(ctx context.Context, id uuid.UUID) (*models.Payment, error) {
	if id == uuid.Nil {
		return nil, errors.New("invalid payment ID")
	}

	payment, err := s.paymentRepo.GetPaymentByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get payment", "error", err, "payment_id", id)
		return nil, errors.New("payment not found")
	}

	return payment, nil
}

// GetPaymentsByTenant retrieves all payments for a tenant
func (s *paymentService) GetPaymentsByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.Payment, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("invalid tenant ID")
	}

	payments, err := s.paymentRepo.GetPaymentsByTenantID(ctx, tenantID)
	if err != nil {
		s.logger.Error("Failed to get payments for tenant", "error", err, "tenant_id", tenantID)
		return nil, errors.New("failed to fetch payments")
	}

	return payments, nil
}

// GetPaymentsByPG retrieves all payments for a PG
func (s *paymentService) GetPaymentsByPG(ctx context.Context, pgID uuid.UUID) ([]models.Payment, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	payments, err := s.paymentRepo.GetPaymentsByPGID(ctx, pgID)
	if err != nil {
		s.logger.Error("Failed to get payments for PG", "error", err, "pg_id", pgID)
		return nil, errors.New("failed to fetch payments")
	}

	return payments, nil
}

// UpdatePaymentStatus updates payment status
func (s *paymentService) UpdatePaymentStatus(ctx context.Context, paymentID uuid.UUID, status string) error {
	if paymentID == uuid.Nil {
		return errors.New("invalid payment ID")
	}

	validStatuses := []string{"pending", "verified", "rejected"}
	isValid := false
	for _, v := range validStatuses {
		if v == status {
			isValid = true
			break
		}
	}

	if !isValid {
		return errors.New("invalid payment status")
	}

	// Get payment to verify it exists
	_, err := s.paymentRepo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return errors.New("payment not found")
	}

	if err := s.paymentRepo.UpdateStatus(ctx, paymentID, status, ""); err != nil {
		s.logger.Error("Failed to update payment status", "error", err, "payment_id", paymentID)
		return errors.New("failed to update payment")
	}

	s.logger.Info("Payment status updated", "payment_id", paymentID, "status", status)
	return nil
}

// VerifyPayment marks payment as verified
func (s *paymentService) VerifyPayment(ctx context.Context, paymentID uuid.UUID, adminRemarks string) error {
	if paymentID == uuid.Nil {
		return errors.New("invalid payment ID")
	}

	payment, err := s.paymentRepo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return errors.New("payment not found")
	}

	if payment.Status != "pending" {
		return errors.New("only pending payments can be verified")
	}

	if err := s.paymentRepo.UpdateStatus(ctx, paymentID, "verified", adminRemarks); err != nil {
		s.logger.Error("Failed to verify payment", "error", err, "payment_id", paymentID)
		return errors.New("failed to verify payment")
	}

	s.logger.Info("Payment verified", "payment_id", paymentID, "tenant_id", payment.TenantID)
	return nil
}

// RejectPayment marks payment as rejected
func (s *paymentService) RejectPayment(ctx context.Context, paymentID uuid.UUID, reason string) error {
	if paymentID == uuid.Nil {
		return errors.New("invalid payment ID")
	}

	payment, err := s.paymentRepo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return errors.New("payment not found")
	}

	if payment.Status != "pending" {
		return errors.New("only pending payments can be rejected")
	}

	if reason == "" {
		reason = "No reason provided"
	}

	if err := s.paymentRepo.UpdateStatus(ctx, paymentID, "rejected", reason); err != nil {
		s.logger.Error("Failed to reject payment", "error", err, "payment_id", paymentID)
		return errors.New("failed to reject payment")
	}

	s.logger.Info("Payment rejected", "payment_id", paymentID, "reason", reason)
	return nil
}

// GetPendingPayments retrieves all pending payments for a PG
func (s *paymentService) GetPendingPayments(ctx context.Context, pgID uuid.UUID) ([]models.Payment, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	payments, err := s.paymentRepo.GetPaymentsByPGID(ctx, pgID)
	if err != nil {
		return nil, errors.New("failed to fetch payments")
	}

	// Filter pending payments
	var pendingPayments []models.Payment
	for _, p := range payments {
		if p.Status == "pending" {
			pendingPayments = append(pendingPayments, p)
		}
	}

	return pendingPayments, nil
}

// GetPaymentStats provides payment statistics for a PG
func (s *paymentService) GetPaymentStats(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	payments, err := s.paymentRepo.GetPaymentsByPGID(ctx, pgID)
	if err != nil {
		return nil, errors.New("failed to fetch payments")
	}

	stats := s.calculatePaymentStats(payments)
	return stats, nil
}

// GetTenantPaymentHistory retrieves payment history for a tenant (last N months)
func (s *paymentService) GetTenantPaymentHistory(ctx context.Context, tenantID uuid.UUID, months int) ([]models.Payment, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("invalid tenant ID")
	}

	if months <= 0 {
		months = 6 // Default 6 months
	}

	payments, err := s.paymentRepo.GetPaymentsByTenantID(ctx, tenantID)
	if err != nil {
		return nil, errors.New("failed to fetch payments")
	}

	// Filter payments from last N months
	cutoffDate := time.Now().AddDate(0, -months, 0)
	var recentPayments []models.Payment

	for _, p := range payments {
		if p.PaymentDate.After(cutoffDate) {
			recentPayments = append(recentPayments, p)
		}
	}

	return recentPayments, nil
}

// GetMonthlyCollectionStats provides monthly collection statistics
func (s *paymentService) GetMonthlyCollectionStats(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	payments, err := s.paymentRepo.GetPaymentsByPGID(ctx, pgID)
	if err != nil {
		return nil, errors.New("failed to fetch payments")
	}

	// Group payments by month
	monthlyStats := make(map[string]map[string]interface{})
	currentMonth := time.Now().Format("2006-01")

	for _, p := range payments {
		month := p.PaymentDate.Format("2006-01")
		if _, exists := monthlyStats[month]; !exists {
			monthlyStats[month] = map[string]interface{}{
				"total_collected": 0.0,
				"total_pending":   0.0,
				"total_rejected":  0.0,
				"count_verified":  0,
				"count_pending":   0,
			}
		}

		stats := monthlyStats[month]
		switch p.Status {
		case "verified":
			stats["total_collected"] = stats["total_collected"].(float64) + p.Amount
			stats["count_verified"] = stats["count_verified"].(int) + 1
		case "pending":
			stats["total_pending"] = stats["total_pending"].(float64) + p.Amount
			stats["count_pending"] = stats["count_pending"].(int) + 1
		case "rejected":
			stats["total_rejected"] = stats["total_rejected"].(float64) + p.Amount
		}
	}

	return map[string]interface{}{
		"monthly_stats": monthlyStats,
		"current_month": currentMonth,
	}, nil
}

// GetOutstandingPayments retrieves all outstanding/rejected payments
func (s *paymentService) GetOutstandingPayments(ctx context.Context, pgID uuid.UUID) ([]models.Payment, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	payments, err := s.paymentRepo.GetPaymentsByPGID(ctx, pgID)
	if err != nil {
		return nil, errors.New("failed to fetch payments")
	}

	// Filter outstanding payments (pending or rejected)
	var outstanding []models.Payment
	for _, p := range payments {
		if p.Status == "pending" || p.Status == "rejected" {
			outstanding = append(outstanding, p)
		}
	}

	return outstanding, nil
}

// GetMonthlyOutstanding checks if tenant has outstanding payment for a month
func (s *paymentService) GetMonthlyOutstanding(ctx context.Context, tenantID uuid.UUID, month string) (*models.Payment, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("invalid tenant ID")
	}
	if month == "" {
		return nil, errors.New("month is required")
	}

	payments, err := s.paymentRepo.GetPaymentsByTenantID(ctx, tenantID)
	if err != nil {
		return nil, errors.New("failed to fetch payments")
	}

	for _, p := range payments {
		if p.ForMonth == month && (p.Status == "pending" || p.Status == "rejected") {
			return &p, nil
		}
	}

	return nil, nil
}

// InitiateMonthlyPayment creates a monthly rent payment record
func (s *paymentService) InitiateMonthlyPayment(ctx context.Context, tenantID uuid.UUID, month string, amount float64) error {
	if tenantID == uuid.Nil {
		return errors.New("invalid tenant ID")
	}
	if month == "" {
		return errors.New("month is required")
	}
	if amount <= 0 {
		return errors.New("amount must be greater than 0")
	}

	// Get tenant to fetch pg_id
	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		return errors.New("tenant not found")
	}

	// Check if payment already exists for this month
	outstanding, _ := s.GetMonthlyOutstanding(ctx, tenantID, month)
	if outstanding != nil {
		return errors.New("payment already exists for this month")
	}

	payment := &models.Payment{
		ID:          uuid.New(),
		TenantID:    tenantID,
		PGID:        tenant.PGID,
		Amount:      amount,
		ForMonth:    month,
		Status:      "pending",
		Method:      "pending",
		PaymentDate: time.Now(),
	}

	if err := s.paymentRepo.CreatePayment(ctx, payment); err != nil {
		s.logger.Error("Failed to initiate monthly payment", "error", err, "tenant_id", tenantID)
		return errors.New("failed to initiate payment")
	}

	s.logger.Info("Monthly payment initiated", "tenant_id", tenantID, "month", month, "amount", amount)
	return nil
}

// Helper function to calculate payment statistics
func (s *paymentService) calculatePaymentStats(payments []models.Payment) map[string]interface{} {
	totalCollected := 0.0
	totalPending := 0.0
	totalRejected := 0.0
	countVerified := 0
	countPending := 0
	countRejected := 0

	for _, p := range payments {
		switch p.Status {
		case "verified":
			totalCollected += p.Amount
			countVerified++
		case "pending":
			totalPending += p.Amount
			countPending++
		case "rejected":
			totalRejected += p.Amount
			countRejected++
		}
	}

	collectionRate := 0.0
	totalAmount := totalCollected + totalPending + totalRejected
	if totalAmount > 0 {
		collectionRate = (totalCollected / totalAmount) * 100
	}

	return map[string]interface{}{
		"total_collected":    totalCollected,
		"total_pending":      totalPending,
		"total_rejected":     totalRejected,
		"count_verified":     countVerified,
		"count_pending":      countPending,
		"count_rejected":     countRejected,
		"collection_rate":    collectionRate,
		"total_transactions": len(payments),
	}
}
