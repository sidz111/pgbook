package handlers

import (
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
	category := c.Param("category")

	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing filename"})
		return
	}

	// Default to tenant-documents if no category specified
	if category == "" {
		category = "tenant-documents"
	}

	filePath, err := h.fileUploadService.ServeSecureUpload(category, filename)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	c.File(filePath)
}
