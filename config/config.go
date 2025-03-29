package config

import (
	"fmt"
	"io/fs"

	env "github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"

	"github.com/pkg/errors"
)

type Config struct {
	App AppConfig
	DB  DBConfig
}

func LoadConfig(appEnv string) (*Config, error) {
	envFile := fmt.Sprintf(".env.%s", appEnv)
	err := godotenv.Load(envFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("error loading environment file %s: %w", envFile, err)
	}

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	if cfg.App.IsDebug {
		fmt.Println("\nLoading environment configuration:")
	}

	// handle app environments
	switch cfg.App.Env {
	case "development":
	case "main":
	case "staging":
	default:
		return nil, errors.Errorf("APP_ENV (=%v) is not set to one of \"development\", \"main\", \"staging\"", cfg.App.Env)
	}

	if cfg.App.IsDebug {
		fmt.Println("APP_")
		fmt.Println("    ENV													 : ", cfg.App.Env)
		fmt.Println("    BASE_ADDR   									 : ", cfg.App.BaseAddr)
		fmt.Println("    DB_DIR  					  			 : ", cfg.DB.Dir)
	}

	return &cfg, nil
}
