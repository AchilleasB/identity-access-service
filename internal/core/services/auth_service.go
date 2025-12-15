package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
	"github.com/golang-jwt/jwt/v5"
)

type GoogleOAuthService struct {
	clientID     string
	clientSecret string
	redirectURL  string
	userRepo     ports.UserRepository
	privateKey   *rsa.PrivateKey
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

func NewGoogleOAuthService(
	clientID, clientSecret, redirectURL string,
	userRepo ports.UserRepository,
	privateKey *rsa.PrivateKey,
) *GoogleOAuthService {
	return &GoogleOAuthService{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		userRepo:     userRepo,
		privateKey:   privateKey,
	}
}

// GenerateState creates a random state for CSRF protection
func (s *GoogleOAuthService) GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GetAuthURL returns the Google authorization URL
func (s *GoogleOAuthService) GetAuthURL(state string) string {
	params := url.Values{
		"client_id":     {s.clientID},
		"redirect_uri":  {s.redirectURL},
		"response_type": {"code"},
		"scope":         {"openid email"},
		"state":         {state},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

// Authenticate exchanges code for tokens, verifies, and returns system JWT
func (s *GoogleOAuthService) Authenticate(ctx context.Context, code string) (string, error) {
	// Exchange code for ID token
	idToken, err := s.exchangeCode(ctx, code)
	if err != nil {
		return "", err
	}

	// Verify ID token and get email
	email, err := s.verifyIDToken(ctx, idToken)
	if err != nil {
		return "", err
	}

	// Find user in database
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return "", errors.New("user not registered")
	}

	// Generate system JWT
	claims := jwt.MapClaims{
		"sub":  user.ID,
		"role": string(user.Role),
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.privateKey)
}

func (s *GoogleOAuthService) exchangeCode(ctx context.Context, code string) (string, error) {
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

func (s *GoogleOAuthService) verifyIDToken(ctx context.Context, idToken string) (string, error) {
	// Fetch Google's public keys
	keys, err := s.fetchGoogleKeys(ctx)
	if err != nil {
		return "", err
	}

	// Parse and verify token
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

	// Verify audience
	// if !claims.VerifyAudience(s.clientID, true) {
	// 	return "", errors.New("invalid audience")
	// }

	// Verify email is present and verified
	if claims.Email == "" || !claims.EmailVerified {
		return "", errors.New("email not verified")
	}

	return claims.Email, nil
}

func (s *GoogleOAuthService) fetchGoogleKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
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
