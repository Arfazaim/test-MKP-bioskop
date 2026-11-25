package middleware

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"bioskop-mkp/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	ContextUserIDKey = "user_id"
	ContextEmailKey  = "email"
	ContextRoleKey   = "role"
)

func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
	secret := []byte(jwtSecret)
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "header authorization tidak ditemukan atau formatnya salah"})
			return
		}

		claims := &model.JWTClaims{}
		token, err := jwt.ParseWithClaims(strings.TrimSpace(parts[1]), claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return secret, nil
		})
		if err != nil || !token.Valid || claims.UserID <= 0 || claims.Role == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token tidak valid atau sudah kedaluwarsa"})
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextEmailKey, claims.Email)
		c.Set(ContextRoleKey, claims.Role)
		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString(ContextRoleKey)
		if role == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "silakan login terlebih dahulu"})
			return
		}
		if !slices.Contains(roles, role) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "akses ditolak: akun anda tidak memiliki izin"})
			return
		}
		c.Next()
	}
}

