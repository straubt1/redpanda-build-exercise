package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/straubt1/redpanda-build-exercise/internal/applog"
	"github.com/straubt1/redpanda-build-exercise/internal/config"
	"github.com/straubt1/redpanda-build-exercise/internal/debugdump"
	"github.com/straubt1/redpanda-build-exercise/internal/githubclient"
	"github.com/straubt1/redpanda-build-exercise/internal/llm"
	"github.com/straubt1/redpanda-build-exercise/internal/worker"
)

func main() {
	if len(os.Args) != 2 {
		applog.Err.Fatalf("usage: reason-test <dir>")
	}
	cfg, err := config.LLMFromEnv()
	if err != nil {
		applog.Err.Fatalf("config: %v", err)
	}
	if err := run(os.Args[1], cfg); err != nil {
		applog.Err.Fatalf("reason-test: %v", err)
	}
}

func run(dir string, cfg config.LLMConfig) error {
	msgRaw, err := os.ReadFile(filepath.Join(dir, "message.json"))
	if err != nil {
		return fmt.Errorf("message.json: %w", err)
	}
	enrRaw, err := os.ReadFile(filepath.Join(dir, "enrichment.json"))
	if err != nil {
		return fmt.Errorf("enrichment.json: %w", err)
	}

	msg, ok := worker.ParseMessage(msgRaw)
	if !ok {
		return fmt.Errorf("invalid or incomplete message.json")
	}
	var enr githubclient.Enrichment
	if err := json.Unmarshal(enrRaw, &enr); err != nil {
		return fmt.Errorf("enrichment.json: %w", err)
	}

	resultsDir := filepath.Join(dir, "results")
	if err := os.RemoveAll(resultsDir); err != nil {
		return fmt.Errorf("results: %w", err)
	}
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
		return fmt.Errorf("results: %w", err)
	}
	debugdump.SetRoot(resultsDir)

	row, _ := worker.Process(context.Background(), llm.New(cfg.OllamaURL, cfg.OllamaModel), msg, &enr, cfg.ConfidenceThreshold)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(row); err != nil {
		return fmt.Errorf("outcome.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(resultsDir, "outcome.json"), buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("outcome.json: %w", err)
	}

	conf := ""
	if row.Confidence != nil {
		conf = fmt.Sprintf("%g", *row.Confidence)
	}
	applog.Info.Printf("event_id=%s category=%s source=%s confidence=%s title=%q",
		row.EventID, row.Category, row.Source, conf, row.Title)
	return nil
}
