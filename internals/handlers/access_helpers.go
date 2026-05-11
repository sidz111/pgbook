package handlers

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"github.com/sidz111/pgbook/internals/services"
)

func getAuthUser(c *gin.Context) (uuid.UUID, string, error) {
	userIDValue, ok := c.Get("userID")
	if !ok {
		return uuid.Nil, "", errors.New("missing user ID in context")
	}

	userIDStr, ok := userIDValue.(string)
	if !ok {
		return uuid.Nil, "", errors.New("invalid user ID type")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, "", err
	}

	roleValue, _ := c.Get("role")
	role, _ := roleValue.(string)

	return userID, role, nil
}

func isAdmin(c *gin.Context) bool {
	_, role, err := getAuthUser(c)
	return err == nil && role == models.RoleAdmin
}

func verifyPGOwnerOrAdmin(c *gin.Context, pgSvc services.PGService, pgID uuid.UUID) bool {
	if pgID == uuid.Nil {
		return false
	}

	userID, role, err := getAuthUser(c)
	if err != nil {
		return false
	}

	if role == models.RoleAdmin {
		return true
	}

	if role != models.RoleOwner {
		return false
	}

	pg, err := pgSvc.GetPGByID(c.Request.Context(), pgID)
	if err != nil {
		return false
	}

	return pg.UserID == userID
}

func verifyTenantOrPGAccess(c *gin.Context, tenant *models.Tenant, pgSvc services.PGService) bool {
	if tenant == nil {
		return false
	}

	if isAdmin(c) {
		return true
	}

	userID, role, err := getAuthUser(c)
	if err != nil {
		return false
	}

	if role == models.RoleTenant {
		return tenant.UserID == userID
	}

	return verifyPGOwnerOrAdmin(c, pgSvc, tenant.PGID)
}

func verifyPGTenantOrAdmin(c *gin.Context, pgSvc services.PGService, tenantSvc services.TenantService, pgID uuid.UUID) bool {
	if pgID == uuid.Nil {
		return false
	}

	if isAdmin(c) {
		return true
	}

	userID, role, err := getAuthUser(c)
	if err != nil {
		return false
	}

	if role == models.RoleOwner {
		pg, err := pgSvc.GetPGByID(c.Request.Context(), pgID)
		if err != nil {
			return false
		}
		return pg.UserID == userID
	}

	if role == models.RoleTenant {
		_, err := tenantSvc.GetTenantByUserIDAndPG(c.Request.Context(), userID, pgID)
		return err == nil
	}

	return false
}

func verifyTenantSelfOrOwner(c *gin.Context, tenant *models.Tenant, pgSvc services.PGService) bool {
	return verifyTenantOrPGAccess(c, tenant, pgSvc)
}
