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
	GetProfile(ctx context.Context, userID uuid.UUID) (*models.User, error)
}

type authService struct {
	userRepo repositories.UserRepository
	secret   string
}

func NewAuthService(repo repositories.UserRepository, secret string) AuthService {
	return &authService{userRepo: repo, secret: secret}
}

func (s *authService) Register(ctx context.Context, user *models.User) error {
	if s.userRepo.EmailExists(ctx, user.Email) {
		return errors.New("email already registered")
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hashedPassword)
	return s.userRepo.CreateUser(ctx, user)
}

func (s *authService) Login(ctx context.Context, email, password string) (string, string, *models.User, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", "", nil, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", "", nil, errors.New("invalid credentials")
	}

	accessToken, err := s.generateToken(user.ID.String(), user.Role, time.Minute*15)
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
	token, err := jwt.Parse(refreshToken, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.secret), nil
	})

	if err != nil || !token.Valid {
		return "", errors.New("invalid refresh token")
	}

	claims := token.Claims.(jwt.MapClaims)
	userIDStr := claims["sub"].(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return "", errors.New("invalid user id in token")
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil || user.RefreshToken != refreshToken {
		return "", errors.New("token mismatch or user not found")
	}

	return s.generateToken(user.ID.String(), user.Role, time.Minute*15)
}

func (s *authService) Logout(ctx context.Context, userID string) error {
	uID, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	user, err := s.userRepo.GetUserByID(ctx, uID)
	if err != nil {
		return err
	}
	user.RefreshToken = ""
	return s.userRepo.UpdateUser(ctx, user)
}

func (s *authService) GetProfile(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	if userID == uuid.Nil {
		return nil, errors.New("invalid user ID")
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	return user, nil
}

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
