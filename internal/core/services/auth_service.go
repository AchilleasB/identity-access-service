package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/domain"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type AuthService struct {
	clientID     string
	clientSecret string
	redirectURL  string
	userRepo     ports.UserRepository
	privateKey   *rsa.PrivateKey
	redisClient  *redis.Client
}

type googleTokenResponse struct {
	IDToken string `json:"id_token"`
}

type googleClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	jwt.RegisteredClaims
}

type googleJWKS struct {
	Keys []struct {
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

type RedisSession struct {
	JTI string `json:"jti"`
	Exp int64  `json:"exp"`
}

const TokenDuration = 30 * time.Minute

func NewAuthService(
	clientID, clientSecret, redirectURL string,
	userRepo ports.UserRepository,
	privateKey *rsa.PrivateKey,
	redisClient *redis.Client,
) *AuthService {
	return &AuthService{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		userRepo:     userRepo,
		privateKey:   privateKey,
		redisClient:  redisClient,
	}
}

// GenerateState creates a random state for CSRF protection
func (s *AuthService) GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GetAuthURL returns the Google authorization URL
func (s *AuthService) GetAuthURL(state string) string {
	params := url.Values{
		"client_id":     []string{s.clientID},
		"redirect_uri":  []string{s.redirectURL},
		"response_type": []string{"code"},
		"scope":         []string{"openid email"},
		"state":         []string{state},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

// Authenticate exchanges code for tokens, verifies, and returns system JWT
func (s *AuthService) Authenticate(ctx context.Context, code string) (string, error) {
	idToken, err := s.exchangeCode(ctx, code)
	if err != nil {
		return "", err
	}

	email, err := s.verifyIDToken(ctx, idToken)
	if err != nil {
		return "", err
	}

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return "", errors.New("user not registered")
	}

	if user.Role == domain.RoleParent {
		status, err := s.userRepo.GetParentStatus(ctx, user.ID)
		if err != nil {
			return "", err
		}
		if domain.ParentStatus(status) == domain.ParentDischarged {
			return "", errors.New("parent is discharged")
		}
	}

	jti := uuid.New().String()
	expTime := time.Now().Add(TokenDuration)

	claims := jwt.MapClaims{
		"sub":  user.ID,
		"role": string(user.Role),
		"jti":  jti,
		"iat":  time.Now().Unix(),
		"exp":  expTime.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", err
	}

	session := RedisSession{JTI: jti, Exp: expTime.Unix()}
	data, _ := json.Marshal(session)

	err = s.redisClient.Set(ctx, "active_session:"+user.ID, data, TokenDuration).Err()
	if err != nil {
		log.Printf("Warning: failed to store active session in redis: %v", err)
	}

	return signedToken, nil
}

func (s *AuthService) Logout(ctx context.Context, tokenString string) error {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return errors.New("invalid claims")
	}

	jti, _ := claims["jti"].(string)
	expTime, _ := claims["exp"].(float64)

	return s.revokeToken(ctx, jti, int64(expTime))
}

func (s *AuthService) DischargeParent(ctx context.Context, parentID string) error {
	metadata, err := s.redisClient.Get(ctx, "active_session:"+parentID).Result()
	if err == redis.Nil {
		return nil
	} else if err != nil {
		return err
	}

	var session RedisSession
	if err := json.Unmarshal([]byte(metadata), &session); err != nil {
		return err
	}

	err = s.revokeToken(ctx, session.JTI, session.Exp)
	if err != nil {
		return err
	}

	err = s.redisClient.Del(ctx, "active_session:"+parentID).Err()
	if err != nil {
		return err
	}

	return s.userRepo.UpdateParentStatus(ctx, parentID)
}

// Internal helper to blacklist a specific JTI
func (s *AuthService) revokeToken(ctx context.Context, jti string, expTime int64) error {
	expirationTime := time.Unix(expTime, 0)
	ttl := time.Until(expirationTime)
	if ttl <= 0 {
		return nil
	}
	return s.redisClient.Set(ctx, "blacklist:"+jti, "revoked", ttl).Err()
}

func (s *AuthService) exchangeCode(ctx context.Context, code string) (string, error) {
	data := url.Values{
		"client_id":     {s.clientID},
		"client_secret": {s.clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {s.redirectURL},
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result googleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.IDToken == "" {
		return "", errors.New("no id_token in response")
	}

	return result.IDToken, nil
}

func (s *AuthService) verifyIDToken(ctx context.Context, idToken string) (string, error) {
	keys, err := s.fetchGoogleKeys(ctx)
	if err != nil {
		return "", err
	}

	token, err := jwt.ParseWithClaims(idToken, &googleClaims{}, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		key, ok := keys[kid]
		if !ok {
			return nil, errors.New("key not found")
		}
		return key, nil
	})
	if err != nil {
		return "", err
	}

	claims := token.Claims.(*googleClaims)

	if claims.Email == "" || !claims.EmailVerified {
		return "", errors.New("email not verified")
	}

	return claims.Email, nil
}

func (s *AuthService) fetchGoogleKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v3/certs", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jwks googleJWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, err
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, k := range jwks.Keys {
		nBytes, _ := base64.RawURLEncoding.DecodeString(k.N)
		eBytes, _ := base64.RawURLEncoding.DecodeString(k.E)

		var e int
		for _, b := range eBytes {
			e = e<<8 + int(b)
		}

		keys[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: e,
		}
	}

	return keys, nil
}
