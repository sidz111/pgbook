package handlers

import (
	"net/http"

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
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role" binding:"required,oneof=admin owner tenant"`
}

// Login Request Struct
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := &models.User{
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	}

	if err := h.service.Register(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Registration successful"})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accessToken, refreshToken, user, err := h.service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 1. Access Token Cookie (Short lived - 15 mins)
	// Parameters: Name, Value, MaxAge (seconds), Path, Domain, Secure, HttpOnly
	c.SetCookie("access_token", accessToken, 900, "/", "", false, true)

	// 2. Refresh Token Cookie (Long lived - 7 days)
	c.SetCookie("refresh_token", refreshToken, 604800, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"user":    user,
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	// Cookie madhun refresh token kadha
	cookieToken, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh token missing"})
		return
	}

	newAccessToken, err := h.service.RefreshToken(c.Request.Context(), cookieToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	// Navin Access Token parat cookie madhe set kara
	c.SetCookie("access_token", newAccessToken, 900, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"message": "Token refreshed successfully"})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// Cookies clear karne (MaxAge = -1 mhanje tabadtob expire)
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("refresh_token", "", -1, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}
