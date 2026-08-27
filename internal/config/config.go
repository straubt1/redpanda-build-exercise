package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	KafkaBrokers    []string
	KafkaTopic      string
	KafkaGroup      string
	PostgresDSN     string
	GitHubToken     string
	GitHubUserAgent string
	OllamaURL       string
	OllamaModel     string
}

func FromEnv() (Config, error) {
	cfg := Config{
		KafkaTopic:      getenv("KAFKA_TOPIC", "github.pr.opened"),
		KafkaGroup:      getenv("KAFKA_GROUP", "pr-triage-worker"),
		PostgresDSN:     getenv("POSTGRES_DSN", "postgres://triage:triage@localhost:5432/triage?sslmode=disable"),
		GitHubToken:     os.Getenv("GITHUB_TOKEN"),
		GitHubUserAgent: getenv("GITHUB_USER_AGENT", "redpanda-build-exercise"),
		OllamaURL:       getenv("OLLAMA_URL", "http://127.0.0.1:11434"),
		OllamaModel:     getenv("OLLAMA_MODEL", "qwen2.5:14b"),
	}
	if cfg.GitHubToken == "" {
		return Config{}, fmt.Errorf("GITHUB_TOKEN is empty")
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

type ServeConfig struct {
	PostgresDSN string
	HTTPAddr    string
}

func ServeFromEnv() (ServeConfig, error) {
	cfg := ServeConfig{
		PostgresDSN: getenv("POSTGRES_DSN", "postgres://triage:triage@localhost:5432/triage?sslmode=disable"),
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
	}
	if cfg.PostgresDSN == "" {
		return ServeConfig{}, fmt.Errorf("POSTGRES_DSN is empty")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
