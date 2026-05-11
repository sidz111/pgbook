package handlers

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sidz111/pgbook/internals/models"
	"github.com/sidz111/pgbook/internals/services"
)

type AuthHandler struct {
	service services.AuthService
}

func NewAuthHandler(service services.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

// Register Request Struct
type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Role     string `json:"role" binding:"required,oneof=admin pg_owner tenant owner"`
}

// Login Request Struct
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Convert owner -> pg_owner
	if req.Role == "owner" {
		req.Role = models.RoleOwner
	}

	// Validate role safety
	validRoles := map[string]bool{
		models.RoleAdmin:  true,
		models.RoleOwner:  true,
		models.RoleTenant: true,
	}

	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid role",
		})
		return
	}

	user := &models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	}

	if err := h.service.Register(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Auto login after registration
	accessToken, refreshToken, createdUser, err := h.service.Login(
		c.Request.Context(),
		req.Email,
		req.Password,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "registration succeeded but failed to create session",
		})
		return
	}

	// Secure cookies only in production
	secureCookie := os.Getenv("ENV") == "production"

	// Set auth cookies
	c.SetCookie("access_token", accessToken, 900, "/", "", secureCookie, true)
	c.SetCookie("refresh_token", refreshToken, 604800, "/", "", secureCookie, true)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Registration successful",
		"user":    createdUser,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	accessToken, refreshToken, user, err := h.service.Login(
		c.Request.Context(),
		req.Email,
		req.Password,
	)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": err.Error(),
		})
		return
	}

	secureCookie := os.Getenv("ENV") == "production"

	// Access token (15 min)
	c.SetCookie("access_token", accessToken, 900, "/", "", secureCookie, true)

	// Refresh token (7 days)
	c.SetCookie("refresh_token", refreshToken, 604800, "/", "", secureCookie, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"user":    user,
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	// Get refresh token from cookie
	cookieToken, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "refresh token missing",
		})
		return
	}

	newAccessToken, err := h.service.RefreshToken(
		c.Request.Context(),
		cookieToken,
	)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid refresh token",
		})
		return
	}

	secureCookie := os.Getenv("ENV") == "production"

	// Set new access token
	c.SetCookie("access_token", newAccessToken, 900, "/", "", secureCookie, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "Token refreshed successfully",
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	if err := h.service.Logout(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	secureCookie := os.Getenv("ENV") == "production"

	// Clear cookies
	c.SetCookie("access_token", "", -1, "/", "", secureCookie, true)
	c.SetCookie("refresh_token", "", -1, "/", "", secureCookie, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	role := c.GetString("role")

	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
		"role":    role,
	})
}
