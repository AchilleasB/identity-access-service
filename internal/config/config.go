package config

import (
	"os"
)

type Config struct {
	JWTPrivateKey string
	DatabaseURL   string
	Port          string
}

func Load() *Config {
	privateKey := os.Getenv("JWT_PRIVATE_KEY") // Path to the RSA private key file
	if privateKey == "" {
		panic("JWT_PRIVATE_KEY environment variable is required")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		panic("DATABASE_URL environment variable is required")
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
