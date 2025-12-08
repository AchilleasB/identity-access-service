package config

import (
	"crypto/rsa"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

type Config struct {
	JWTPrivateKey *rsa.PrivateKey
	JWTPublicKey  *rsa.PublicKey
	DatabaseURL   string
	Port          string
}

func Load() *Config {
	privateKeyPath := os.Getenv("PRIVATE_KEY_PATH")
	if privateKeyPath == "" {
		privateKeyPath = "/etc/certs/private.pem"
	}
	privateKey, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		panic("Failed to load private key: " + err.Error())
	}

	publicKeyPath := os.Getenv("PUBLIC_KEY_PATH")
	if publicKeyPath == "" {
		publicKeyPath = "/etc/certs/public.pem"
	}
	publicKey, err := loadPublicKey(publicKeyPath)
	if err != nil {
		panic("Failed to load public key: " + err.Error())
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
		JWTPublicKey:  publicKey,
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

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(keyData)
	if err != nil {
		return nil, err
	}
	return publicKey, nil
}
