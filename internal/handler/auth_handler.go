package handler

import (
	"errors"
	"log"
	"net/http"

	"bioskop-mkp/internal/middleware"
	"bioskop-mkp/internal/model"
	"bioskop-mkp/internal/repository"
	"bioskop-mkp/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nama lengkap, email yang valid, dan password minimal 6 karakter wajib diisi"})
		return
	}

	user, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, repository.ErrEmailAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			log.Printf("register error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "terjadi kesalahan pada server"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "pendaftaran akun berhasil",
		"data":    user,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email dan password wajib diisi dengan format yang benar"})
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		log.Printf("login error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "terjadi kesalahan pada server"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "berhasil login",
		"data":    resp,
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetInt64(middleware.ContextUserIDKey)
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "silakan login terlebih dahulu"})
		return
	}

	user, err := h.authService.GetProfile(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		log.Printf("get profile error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "terjadi kesalahan pada server"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "berhasil mengambil profil",
		"data":    user,
	})
}
