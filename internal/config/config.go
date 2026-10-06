package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Environment string `env:"ENVIRONMENT"`
	Port        string `env:"API_PORT"`
	DatabaseURL string `env:"DATABASE_URL"`
	AWSREGION   string `env:"AWS_REGION"`
	SQSEndpoint string `env:"SQS_ENDPOINT"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	return &cfg, nil
}
