package config

import (
	"crypto/rsa"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

type Config struct {
	JWTPrivateKey *rsa.PrivateKey
	DatabaseURL   string
	Port          string
}

func Load() *Config {
	keyPath := os.Getenv("PRIVATE_KEY_PATH")
	if keyPath == "" {
		keyPath = "/etc/certs/private.pem"
	}

	privateKey, err := loadPrivateKey(keyPath)
	if err != nil {
		panic("Failed to load private key: " + err.Error())
	}

	dbURL := os.Getenv("DB_CONNECTION_STRING")
	if dbURL == "" {
		panic("DB_CONNECTION_STRING environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		JWTPrivateKey: privateKey,
		DatabaseURL:   dbURL,
		Port:          port,
	}
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(keyData)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}
