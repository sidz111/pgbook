package services

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"github.com/sidz111/pgbook/internals/repositories"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Register(ctx context.Context, user *models.User) error
	Login(ctx context.Context, email, password string) (string, string, *models.User, error)
	RefreshToken(ctx context.Context, refreshToken string) (string, error)
	Logout(ctx context.Context, userID string) error
}

type authService struct {
	userRepo repositories.UserRepository
	secret   string
}

func NewAuthService(repo repositories.UserRepository, secret string) AuthService {
	return &authService{userRepo: repo, secret: secret}
}

func (s *authService) Register(ctx context.Context, user *models.User) error {
	// 1. Check if email already exists
	if s.userRepo.EmailExists(ctx, user.Email) {
		return errors.New("email already registered")
	}

	// 2. Hash Password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hashedPassword)

	// 3. Save to DB
	return s.userRepo.CreateUser(ctx, user)
}

// Login: Verify credentials aani Access/Refresh token dene
func (s *authService) Login(ctx context.Context, email, password string) (string, string, *models.User, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", "", nil, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", "", nil, errors.New("invalid credentials")
	}

	// Access Token (15 Minutes)
	accessToken, err := s.generateToken(user.ID.String(), user.Role, time.Minute*15)
	if err != nil {
		return "", "", nil, err
	}

	// Refresh Token (7 Days)
	refreshToken, err := s.generateToken(user.ID.String(), user.Role, time.Hour*24*7)
	if err != nil {
		return "", "", nil, err
	}

	user.RefreshToken = refreshToken
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return "", "", nil, err
	}

	return accessToken, refreshToken, user, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	// 1. Token Parse and Verify
	token, err := jwt.Parse(refreshToken, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.secret), nil
	})

	if err != nil || !token.Valid {
		return "", errors.New("invalid refresh token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid token claims")
	}

	userIDStr := claims["sub"].(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return "", errors.New("invalid user id in token")
	}

	// 2. check in db if token matches with the one stored for the user
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil || user.RefreshToken != refreshToken {
		return "", errors.New("token mismatch or user not found")
	}

	// 3. New access token (15 mins)
	newAccessToken, err := s.generateToken(user.ID.String(), user.Role, time.Minute*15)
	return newAccessToken, err
}

// Logout: creare token from DB and clear cookie from client side
func (s *authService) Logout(ctx context.Context, userID string) error {
	uID, err := uuid.Parse(userID)
	if err != nil {
		return errors.New("invalid user id")
	}

	user, err := s.userRepo.GetUserByID(ctx, uID)
	if err != nil {
		return err
	}

	user.RefreshToken = "" // Clear token from DB
	return s.userRepo.UpdateUser(ctx, user)
}

// Helper: Token generation logic
func (s *authService) generateToken(userID, role string, expiry time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(expiry).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}
