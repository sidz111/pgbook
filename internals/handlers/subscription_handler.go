package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"github.com/sidz111/pgbook/internals/services"
	"github.com/sidz111/pgbook/internals/utils"
)

type SubscriptionHandler struct {
	subscriptionService services.SubscriptionService
	pgService           services.PGService
	fileUploadService   *utils.FileUploadService
	logger              *slog.Logger
}

func NewSubscriptionHandler(subscriptionService services.SubscriptionService, pgService services.PGService, fileUploadService *utils.FileUploadService) *SubscriptionHandler {
	return &SubscriptionHandler{
		subscriptionService: subscriptionService,
		pgService:           pgService,
		fileUploadService:   fileUploadService,
		logger:              slog.Default(),
	}
}

type CreateSubscriptionRequest struct {
	PGID     string  `json:"pg_id" binding:"required"`
	Amount   float64 `json:"amount" binding:"required,gt=0"`
	ProofURL string  `json:"proof_url" binding:"required"`
	PlanName string  `json:"plan_name"`
}

type ApproveSubscriptionRequest struct {
	Months int `json:"months" binding:"omitempty,gt=0"`
}

func (h *SubscriptionHandler) CreateSubscription(c *gin.Context) {
	var req CreateSubscriptionRequest
	proofURL := ""

	if strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
		if file, err := c.FormFile("proof"); err == nil {
			uploadedURL, err := h.fileUploadService.UploadTenantDocument(file, "subscription_proof")
			if err != nil {
				h.logger.Error("Failed to upload subscription proof", "error", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			proofURL = uploadedURL
		}
		req.PGID = c.PostForm("pg_id")
		req.PlanName = c.PostForm("plan_name")
		req.ProofURL = c.PostForm("proof_url")
		amountStr := c.PostForm("amount")
		if amountStr != "" {
			amount, err := strconv.ParseFloat(amountStr, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount format"})
				return
			}
			req.Amount = amount
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			h.logger.Error("Subscription binding error", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "validation failed: " + err.Error(),
				"details": map[string]interface{}{
					"required_fields":             []string{"pg_id", "amount", "proof_url"},
					"amount_must_be_greater_than": 0,
				},
			})
			return
		}
	}

	if proofURL == "" {
		proofURL = req.ProofURL
	}

	// Additional validation
	if req.PGID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pg_id is required"})
		return
	}
	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be greater than 0"})
		return
	}
	if proofURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "proof_url (payment screenshot URL) or proof file is required"})
		return
	}
	req.ProofURL = proofURL

	pgID, err := uuid.Parse(req.PGID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID format"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	subscription := &models.Subscription{
		PGID:     pgID,
		Amount:   req.Amount,
		ProofURL: req.ProofURL,
		PlanName: req.PlanName,
	}

	if err := h.subscriptionService.CreateSubscription(c.Request.Context(), subscription); err != nil {
		h.logger.Error("Failed to create subscription", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Subscription request created"})
}

func (h *SubscriptionHandler) ApproveSubscription(c *gin.Context) {
	subID, err := uuid.Parse(c.Param("sub_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription ID"})
		return
	}

	var req ApproveSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if !errors.Is(err, io.EOF) {
			h.logger.Error("Subscription approve binding error", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if req.Months <= 0 {
		req.Months = 1
	}

	userID, _, err := getAuthUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
		return
	}

	if err := h.subscriptionService.ApproveSubscription(c.Request.Context(), subID, req.Months, userID.String()); err != nil {
		h.logger.Error("Failed to approve subscription", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription approved"})
}

func (h *SubscriptionHandler) RejectSubscription(c *gin.Context) {
	subID, err := uuid.Parse(c.Param("sub_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription ID"})
		return
	}

	if err := h.subscriptionService.RejectSubscription(c.Request.Context(), subID); err != nil {
		h.logger.Error("Failed to reject subscription", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription rejected"})
}

func (h *SubscriptionHandler) GetSubscriptionsByPG(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	subscriptions, err := h.subscriptionService.GetSubscriptionsByPG(c.Request.Context(), pgID)
	if err != nil {
		h.logger.Error("Failed to list subscriptions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"subscriptions": subscriptions, "count": len(subscriptions)})
}

func (h *SubscriptionHandler) GetSubscriptionByID(c *gin.Context) {
	subID, err := uuid.Parse(c.Param("sub_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription ID"})
		return
	}

	subscription, err := h.subscriptionService.GetSubscriptionByID(c.Request.Context(), subID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, subscription.PGID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	c.JSON(http.StatusOK, subscription)
}

func (h *SubscriptionHandler) GetActiveSubscription(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	subscription, err := h.subscriptionService.GetActiveSubscription(c.Request.Context(), pgID)
	if err != nil {
		h.logger.Error("Failed to get active subscription", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, subscription)
}

func (h *SubscriptionHandler) GetPendingSubscriptions(c *gin.Context) {
	subscriptions, err := h.subscriptionService.GetPendingSubscriptions(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get pending subscriptions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"subscriptions": subscriptions, "count": len(subscriptions)})
}
