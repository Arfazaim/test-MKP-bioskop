package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"bioskop-mkp/internal/model"
	"bioskop-mkp/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("email atau password salah")
	ErrInvalidInput       = errors.New("data yang dikirim tidak lengkap atau tidak sesuai")
)

type AuthService interface {
	Register(ctx context.Context, req model.RegisterRequest) (*model.User, error)
	Login(ctx context.Context, req model.LoginRequest) (*model.AuthResponse, error)
	GetProfile(ctx context.Context, userID int64) (*model.User, error)
}

type authService struct {
	userRepo       repository.UserRepository
	jwtSecret      []byte
	jwtExpireHours int
}

func NewAuthService(userRepo repository.UserRepository, jwtSecret string, jwtExpireHours int) AuthService {
	return &authService{
		userRepo:       userRepo,
		jwtSecret:      []byte(jwtSecret),
		jwtExpireHours: jwtExpireHours,
	}
}

func (s *authService) Register(ctx context.Context, req model.RegisterRequest) (*model.User, error) {
	fullName := strings.TrimSpace(req.FullName)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if fullName == "" || email == "" || len(req.Password) < 6 || len(req.Password) > 72 {
		return nil, ErrInvalidInput
	}

	var phone *string
	if req.Phone != nil {
		trimmed := strings.TrimSpace(*req.Phone)
		if trimmed != "" {
			phone = &trimmed
		}
	}

	if _, err := s.userRepo.FindByEmail(ctx, email); err == nil {
		return nil, repository.ErrEmailAlreadyExists
	} else if !errors.Is(err, repository.ErrUserNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		FullName:     fullName,
		Email:        email,
		PasswordHash: string(hash),
		Phone:        phone,
		Role:         model.RoleCustomer,
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *authService) Login(ctx context.Context, req model.LoginRequest) (*model.AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(s.jwtExpireHours) * time.Hour)
	claims := model.JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(user.ID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		Token:     signedToken,
		TokenType: "Bearer",
		ExpiresAt: expiresAt,
		User:      user,
	}, nil
}

func (s *authService) GetProfile(ctx context.Context, userID int64) (*model.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}

