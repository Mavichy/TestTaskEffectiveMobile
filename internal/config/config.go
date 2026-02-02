package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr string
	DBDSN    string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	httpAddr := strings.TrimSpace(os.Getenv("HTTP_ADDR"))
	if httpAddr == "" {
		httpAddr = ":8080"
	}

	dsn := strings.TrimSpace(os.Getenv("DB_DSN"))
	if dsn == "" {
		return Config{}, errors.New("DB_DSN is required")
	}

	return Config{
		HTTPAddr: httpAddr,
		DBDSN:    dsn,
	}, nil
}
