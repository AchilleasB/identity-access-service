package middleware

import (
	"context"
	"crypto/rsa"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type AuthMiddleware struct {
	publicKey *rsa.PublicKey
}

func NewAuthMiddleware(publicKey *rsa.PublicKey) *AuthMiddleware {
	return &AuthMiddleware{
		publicKey: publicKey,
	}
}

type contextKey string

const (
	UserIDKey contextKey = "userID"
	RoleKey   contextKey = "role"
)

func (m *AuthMiddleware) RequireRole(roles []string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract token from header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Printf("Missing Authorization header")
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			log.Printf("Invalid Authorization header format")
			http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return m.publicKey, nil
		})

		if err != nil {
			log.Printf("Token parse error: %v", err)
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		if !token.Valid {
			log.Printf("Token not valid")
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			log.Printf("Failed to extract claims")
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}

		userID, ok := claims["sub"].(string)
		if !ok || userID == "" {
			log.Printf("Missing or invalid 'sub' claim: %v", claims["sub"])
			http.Error(w, "invalid token: missing user ID", http.StatusUnauthorized)
			return
		}

		userRole, ok := claims["role"].(string)
		if !ok || userRole == "" {
			log.Printf("Missing or invalid 'role' claim: %v", claims["role"])
			http.Error(w, "invalid token: missing role", http.StatusUnauthorized)
			return
		}

		log.Printf("Token validated - UserID: %s, Role: %s", userID, userRole)

		allowedRoles := false
		for _, r := range roles {
			if userRole == r {
				allowedRoles = true
				break
			}
		}
		if !allowedRoles {
			log.Printf("Role mismatch: required one of %v, got %s", roles, userRole)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, RoleKey, userRole)

		next(w, r.WithContext(ctx))
	}
}
