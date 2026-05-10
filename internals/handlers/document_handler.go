package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sidz111/pgbook/internals/services"
	"github.com/sidz111/pgbook/internals/utils"
)

type DocumentHandler struct {
	fileUploadService *utils.FileUploadService
	tenantService     services.TenantService
	pgService         services.PGService
	logger            *slog.Logger
}

func NewDocumentHandler(
	fileUploadService *utils.FileUploadService,
	tenantService services.TenantService,
	pgService services.PGService,
) *DocumentHandler {
	return &DocumentHandler{
		fileUploadService: fileUploadService,
		tenantService:     tenantService,
		pgService:         pgService,
		logger:            slog.Default(),
	}
}

func (h *DocumentHandler) ServeDocument(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing filename"})
		return
	}

	documentURL := fmt.Sprintf("/api/v1/documents/%s", filename)
	tenant, err := h.tenantService.GetTenantByDocumentURL(c.Request.Context(), documentURL)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}

	if !verifyTenantOrPGAccess(c, tenant, h.pgService) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access to document"})
		return
	}

	filePath, err := h.fileUploadService.ServeSecureDocument(filename)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}

	c.File(filePath)
}
