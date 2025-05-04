package internal

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Interval          time.Duration
	PostgresDsn       string
	ClickhouseDsn     string
	DiscoveryInterval time.Duration
}

func NewConfig() (*Config, error) {
	i, _ := time.ParseDuration("30s")
	d, _ := time.ParseDuration("30s")
	cfg := &Config{
		Interval:          i,
		DiscoveryInterval: d,
		PostgresDsn:       "postgres://postgres@localhost:5432/postgres?sslmode=disable",
		ClickhouseDsn:     "http://localhost:8123/default",
	}
	if v := os.Getenv("INTERVAL"); v != "" {
		i, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("read params errors: %w", err)
		}
		cfg.Interval = i
	}
	if v := os.Getenv("DISCOVERY_INTERVAL"); v != "" {
		i, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("read params errors: %w", err)
		}
		cfg.DiscoveryInterval = i
	}
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		cfg.PostgresDsn = v
	}
	if v := os.Getenv("CLICKHOUSE_DSN"); v != "" {
		cfg.ClickhouseDsn = v
	}
	return cfg, nil
}
