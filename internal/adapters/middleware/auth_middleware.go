package middleware

import (
	"context"
	"crypto/rsa"
	"log"
	"net/http"
	"strings"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"
)

type AuthMiddleware struct {
	publicKey   *rsa.PublicKey
	redisClient *redis.Client
	redisCB     *gobreaker.CircuitBreaker
}

func NewAuthMiddleware(publicKey *rsa.PublicKey, redisClient *redis.Client) *AuthMiddleware {
	// Configure circuit breaker for Redis operations
	// Fail-closed strategy: When circuit is open, reject requests for security
	redisCB := config.NewCircuitBreaker("Redis-Auth")

	return &AuthMiddleware{
		publicKey:   publicKey,
		redisClient: redisClient,
		redisCB:     redisCB,
	}
}

type ContextKey string

const (
	UserIDKey ContextKey = "userID"
	RoleKey   ContextKey = "role"
	TokenKey  ContextKey = "token"
)

func (m *AuthMiddleware) RequireRole(roles []string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Printf("Missing Authorization header")
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return m.publicKey, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}

		revoked, err := m.isBlacklisted(claims, r.Context())
		if err != nil {
			// Circuit breaker is open or Redis failed - FAIL CLOSED
			log.Printf("[CRITICAL] Authentication service unavailable: %v", err)
			http.Error(w, "authentication service unavailable", http.StatusServiceUnavailable)
			return
		}
		if revoked {
			http.Error(w, "token revoked", http.StatusUnauthorized)
			return
		}

		userID, _ := claims["sub"].(string)
		userRole, _ := claims["role"].(string)

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
		ctx = context.WithValue(ctx, TokenKey, tokenString)

		next(w, r.WithContext(ctx))
	}
}

func (m *AuthMiddleware) isBlacklisted(claims jwt.MapClaims, ctx context.Context) (bool, error) {
	jti, _ := claims["jti"].(string)

	// Execute Redis check through circuit breaker
	result, err := m.redisCB.Execute(func() (interface{}, error) {
		return m.redisClient.Exists(ctx, "blacklist:"+jti).Result()
	})

	if err != nil {
		// Circuit breaker is open or Redis operation failed
		log.Printf("[CRITICAL] Redis blacklist check failed (circuit may be open): %v", err)
		return false, err
	}

	isRevoked, ok := result.(int64)
	return ok && isRevoked > 0, nil
}
