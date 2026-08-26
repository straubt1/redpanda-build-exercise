package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroup   string
	PostgresDSN  string
}

func FromEnv() (Config, error) {
	cfg := Config{
		KafkaTopic:  getenv("KAFKA_TOPIC", "github.pr.opened"),
		KafkaGroup:  getenv("KAFKA_GROUP", "pr-triage-worker"),
		PostgresDSN: getenv("POSTGRES_DSN", "postgres://triage:triage@localhost:5432/triage?sslmode=disable"),
	}

	for _, b := range strings.Split(getenv("KAFKA_BROKERS", "localhost:19092"), ",") {
		b = strings.TrimSpace(b)
		if b != "" {
			cfg.KafkaBrokers = append(cfg.KafkaBrokers, b)
		}
	}
	if len(cfg.KafkaBrokers) == 0 {
		return Config{}, fmt.Errorf("KAFKA_BROKERS is empty")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
