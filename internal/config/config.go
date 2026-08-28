package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	KafkaBrokers        []string
	KafkaTopic          string
	KafkaGroup          string
	PostgresDSN         string
	GitHubToken         string
	GitHubUserAgent     string
	OllamaURL           string
	OllamaModel         string
	MaxNumberFiles      int
	MaxFilePatchSize    int
	ConfidenceThreshold float64
}

func FromEnv() (Config, error) {
	maxFiles, err := getenvInt("REASON_MAX_NUMBER_FILES", 20)
	if err != nil {
		return Config{}, err
	}
	maxPatch, err := getenvInt("REASON_MAX_FILE_PATCH_SIZE", 4000)
	if err != nil {
		return Config{}, err
	}
	threshold, err := getenvFloat("CONFIDENCE_THRESHOLD", 0.6)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		KafkaTopic:          getenv("KAFKA_TOPIC", "github.pr.opened"),
		KafkaGroup:          getenv("KAFKA_GROUP", "pr-triage-worker"),
		PostgresDSN:         getenv("POSTGRES_DSN", "postgres://triage:triage@localhost:5432/triage?sslmode=disable"),
		GitHubToken:         os.Getenv("GITHUB_TOKEN"),
		GitHubUserAgent:     getenv("GITHUB_USER_AGENT", "redpanda-build-exercise"),
		OllamaURL:           getenv("OLLAMA_URL", "http://127.0.0.1:11434"),
		OllamaModel:         getenv("OLLAMA_MODEL", "llama3:8b"),
		MaxNumberFiles:      maxFiles,
		MaxFilePatchSize:    maxPatch,
		ConfidenceThreshold: threshold,
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

type LLMConfig struct {
	OllamaURL           string
	OllamaModel         string
	ConfidenceThreshold float64
}

func LLMFromEnv() (LLMConfig, error) {
	threshold, err := getenvFloat("CONFIDENCE_THRESHOLD", 0.6)
	if err != nil {
		return LLMConfig{}, err
	}
	return LLMConfig{
		OllamaURL:           getenv("OLLAMA_URL", "http://127.0.0.1:11434"),
		OllamaModel:         getenv("OLLAMA_MODEL", "llama3:8b"),
		ConfidenceThreshold: threshold,
	}, nil
}

type ServeConfig struct {
	PostgresDSN string
	HTTPAddr    string
	ListCap     int
}

func ServeFromEnv() (ServeConfig, error) {
	listCap, err := getenvInt("SERVE_LIST_CAP", 20)
	if err != nil {
		return ServeConfig{}, err
	}
	cfg := ServeConfig{
		PostgresDSN: getenv("POSTGRES_DSN", "postgres://triage:triage@localhost:5432/triage?sslmode=disable"),
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		ListCap:     listCap,
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

func getenvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s must be > 0", key)
	}
	return n, nil
}

func getenvFloat(key string, fallback float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if n < 0 || n > 1 {
		return 0, fmt.Errorf("%s must be between 0 and 1", key)
	}
	return n, nil
}
