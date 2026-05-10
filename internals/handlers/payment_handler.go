package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"github.com/sidz111/pgbook/internals/services"
)

type PaymentHandler struct {
	paymentService services.PaymentService
	tenantService  services.TenantService
	pgService      services.PGService
	logger         *slog.Logger
}

func NewPaymentHandler(
	paymentService services.PaymentService,
	tenantService services.TenantService,
	pgService services.PGService,
) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		tenantService:  tenantService,
		pgService:      pgService,
		logger:         slog.Default(),
	}
}

// CreatePaymentRequest defines payment creation request
type CreatePaymentRequest struct {
	TenantID      string  `json:"tenant_id" binding:"required"`
	PGID          string  `json:"pg_id" binding:"required"`
	Amount        float64 `json:"amount" binding:"required"`
	ForMonth      string  `json:"for_month" binding:"required"`      // YYYY-MM format
	PaymentMethod string  `json:"payment_method" binding:"required"` // "cash" or "upi_qr"
	TransactionID string  `json:"transaction_id"`                    // For UPI/QR
}

// CreatePayment - POST /v1/payment/create
// Creates payment record (status: pending initially)
func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	pgID, err := uuid.Parse(req.PGID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	// Verify access
	if !verifyPGOwnerOrAdmin(c, h.pgService, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	// Validate payment method
	if req.PaymentMethod != "cash" && req.PaymentMethod != "upi_qr" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment method"})
		return
	}

	payment := &models.Payment{
		TenantID:      tenantID,
		PGID:          pgID,
		Amount:        req.Amount,
		ForMonth:      req.ForMonth,
		Method:        req.PaymentMethod,
		TransactionID: req.TransactionID,
		Status:        "pending",
	}

	if err := h.paymentService.CreatePayment(c.Request.Context(), payment); err != nil {
		h.logger.Error("Failed to create payment", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Payment created successfully",
		"payment_id": payment.ID,
		"status":     "pending",
	})
}

// VerifyCashPayment - POST /v1/payment/:payment_id/verify-cash
// Owner marks cash payment as received
type VerifyCashPaymentRequest struct {
	Remarks string `json:"remarks"`
}

func (h *PaymentHandler) VerifyCashPayment(c *gin.Context) {
	paymentID, err := uuid.Parse(c.Param("payment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment ID"})
		return
	}

	var req VerifyCashPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payment, err := h.paymentService.GetPaymentByID(c.Request.Context(), paymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
		return
	}

	// Verify access to PG
	if !verifyPGOwnerOrAdmin(c, h.pgService, payment.PGID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	// Validate payment method
	if payment.Method != "cash" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "this payment is not a cash payment"})
		return
	}

	if payment.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only pending payments can be verified"})
		return
	}

	if req.Remarks == "" {
		req.Remarks = "Cash payment verified by owner"
	}

	if err := h.paymentService.VerifyPayment(c.Request.Context(), paymentID, req.Remarks); err != nil {
		h.logger.Error("Failed to verify cash payment", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cash payment verified successfully",
		"status":  "verified",
	})
}

// VerifyUPIPayment - POST /v1/payment/:payment_id/verify-upi
// Owner verifies UPI payment using transaction ID
type VerifyUPIPaymentRequest struct {
	TransactionID string `json:"transaction_id" binding:"required"`
	Remarks       string `json:"remarks"`
}

func (h *PaymentHandler) VerifyUPIPayment(c *gin.Context) {
	paymentID, err := uuid.Parse(c.Param("payment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment ID"})
		return
	}

	var req VerifyUPIPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payment, err := h.paymentService.GetPaymentByID(c.Request.Context(), paymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
		return
	}

	// Verify access
	if !verifyPGOwnerOrAdmin(c, h.pgService, payment.PGID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	// Validate payment method
	if payment.Method != "upi_qr" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "this payment is not a UPI payment"})
		return
	}

	if payment.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only pending payments can be verified"})
		return
	}

	// In production, verify transaction ID against UPI gateway or bank
	// For now, we accept manual verification
	if req.Remarks == "" {
		req.Remarks = "UPI payment verified via transaction: " + req.TransactionID
	}

	// Update transaction ID if provided
	if req.TransactionID != "" {
		payment.TransactionID = req.TransactionID
	}

	if err := h.paymentService.VerifyPayment(c.Request.Context(), paymentID, req.Remarks); err != nil {
		h.logger.Error("Failed to verify UPI payment", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "UPI payment verified successfully",
		"status":  "verified",
		"txn_id":  req.TransactionID,
	})
}

// RejectPayment - POST /v1/payment/:payment_id/reject
type RejectPaymentRequest struct {
	Reason string `json:"reason" binding:"required"`
}

func (h *PaymentHandler) RejectPayment(c *gin.Context) {
	paymentID, err := uuid.Parse(c.Param("payment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment ID"})
		return
	}

	var req RejectPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payment, err := h.paymentService.GetPaymentByID(c.Request.Context(), paymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
		return
	}

	// Only PG owner or admin can reject
	if !verifyPGOwnerOrAdmin(c, h.pgService, payment.PGID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	if err := h.paymentService.RejectPayment(c.Request.Context(), paymentID, req.Reason); err != nil {
		h.logger.Error("Failed to reject payment", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Payment rejected",
		"status":  "rejected",
		"reason":  req.Reason,
	})
}

// GetPaymentStats - GET /v1/pg/:pg_id/payment-stats
func (h *PaymentHandler) GetPaymentStats(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	stats, err := h.paymentService.GetPaymentStats(c.Request.Context(), pgID)
	if err != nil {
		h.logger.Error("Failed to get payment stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetMonthlyStats - GET /v1/pg/:pg_id/monthly-collection
func (h *PaymentHandler) GetMonthlyStats(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	stats, err := h.paymentService.GetMonthlyCollectionStats(c.Request.Context(), pgID)
	if err != nil {
		h.logger.Error("Failed to get monthly stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetPendingPayments - GET /v1/pg/:pg_id/payments/pending
func (h *PaymentHandler) GetPendingPayments(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	payments, err := h.paymentService.GetPendingPayments(c.Request.Context(), pgID)
	if err != nil {
		h.logger.Error("Failed to get pending payments", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pending_payments": payments,
		"count":            len(payments),
	})
}

// GetTenantPayments - GET /v1/tenant/:tenant_id/payments
func (h *PaymentHandler) GetTenantPayments(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("tenant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	tenant, err := h.tenantService.GetTenantByID(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	// Verify access
	if !verifyTenantOrPGAccess(c, tenant, h.pgService) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	months, _ := strconv.Atoi(c.DefaultQuery("months", "6"))
	payments, err := h.paymentService.GetTenantPaymentHistory(c.Request.Context(), tenantID, months)
	if err != nil {
		h.logger.Error("Failed to get tenant payments", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"payments": payments,
		"count":    len(payments),
	})
}

// InitiateMonthlyPayment - POST /v1/tenant/:tenant_id/payment/monthly
type InitiateMonthlyPaymentRequest struct {
	Month  string  `json:"month" binding:"required"` // YYYY-MM format
	Amount float64 `json:"amount" binding:"required"`
}

func (h *PaymentHandler) InitiateMonthlyPayment(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("tenant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	var req InitiateMonthlyPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant, err := h.tenantService.GetTenantByID(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	// PG owner or admin can initiate
	if !verifyPGOwnerOrAdmin(c, h.pgService, tenant.PGID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	if err := h.paymentService.InitiateMonthlyPayment(c.Request.Context(), tenantID, req.Month, req.Amount); err != nil {
		h.logger.Error("Failed to initiate monthly payment", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Monthly payment initiated",
		"month":   req.Month,
		"amount":  req.Amount,
	})
}

// GetUPIQRCode - GET /v1/pg/:pg_id/upi-qr
// Returns UPI QR code for tenant payments
func (h *PaymentHandler) GetUPIQRCode(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	pg, err := h.pgService.GetPGByID(c.Request.Context(), pgID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PG not found"})
		return
	}

	if pg.ScannerURL == "" {
		c.JSON(http.StatusOK, gin.H{
			"message": "UPI QR code not configured",
			"qr_url":  "",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "UPI QR code retrieved",
		"qr_url":  pg.ScannerURL,
		"note":    "Display this QR code to tenant for UPI payment",
	})
}
