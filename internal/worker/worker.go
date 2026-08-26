package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/straubt1/redpanda-build-exercise/internal/applog"
	"github.com/straubt1/redpanda-build-exercise/internal/config"
	"github.com/straubt1/redpanda-build-exercise/internal/githubclient"
	"github.com/straubt1/redpanda-build-exercise/internal/kafka"
	"github.com/straubt1/redpanda-build-exercise/internal/llm"
	"github.com/straubt1/redpanda-build-exercise/internal/reason"
	"github.com/straubt1/redpanda-build-exercise/internal/skipllm"
	"github.com/straubt1/redpanda-build-exercise/internal/store"
)

// Work is the Connect-projected topic payload. Title is not on /events; ignore if present.
type Work struct {
	EventID   string `json:"event_id"`
	Repo      string `json:"repo"`
	PRNumber  int    `json:"pr_number"`
	PRURL     string `json:"pr_url"`
	Author    string `json:"author"`
	Action    string `json:"action"`
	CreatedAt string `json:"created_at"`
}

func Run(ctx context.Context, cfg config.Config) error {
	db, err := store.Connect(ctx, cfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	c, err := kafka.NewConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroup)
	if err != nil {
		return err
	}
	defer c.Close()

	gh := githubclient.New(cfg.GitHubToken, cfg.GitHubUserAgent)
	ollama := llm.New(cfg.OllamaURL, cfg.OllamaModel)

	applog.Info.Printf("consuming topic=%s group=%s brokers=%v ollama=%s model=%s",
		cfg.KafkaTopic, cfg.KafkaGroup, cfg.KafkaBrokers, cfg.OllamaURL, cfg.OllamaModel)

	for {
		records, err := c.Poll(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		for _, rec := range records {
			if err := handle(ctx, db, gh, ollama, rec.Value); err != nil {
				return fmt.Errorf("upsert offset=%d: %w", rec.Offset, err)
			}
			if err := c.Commit(ctx, rec); err != nil {
				return fmt.Errorf("commit offset=%d: %w", rec.Offset, err)
			}
		}
	}
}

func handle(ctx context.Context, db *store.Store, gh *githubclient.Client, ollama reason.Completer, raw []byte) error {
	var msg Work
	if err := json.Unmarshal(raw, &msg); err != nil {
		applog.Err.Printf("skip bad json: %v", err)
		return nil
	}
	if msg.EventID == "" || msg.Repo == "" || msg.PRNumber == 0 {
		applog.Err.Printf("skip incomplete message event_id=%q repo=%q pr=%d", msg.EventID, msg.Repo, msg.PRNumber)
		return nil
	}

	row := store.Row{
		EventID:   msg.EventID,
		Repo:      msg.Repo,
		PRNumber:  msg.PRNumber,
		PRURL:     msg.PRURL,
		Author:    msg.Author,
		Action:    msg.Action,
		Category:  "unknown",
		Source:    "fallback",
		Rationale: "pending enrichment",
	}
	if t, err := time.Parse(time.RFC3339, msg.CreatedAt); err == nil {
		row.ReceivedAt = &t
	}

	enr, err := gh.Fetch(ctx, msg.Repo, msg.PRNumber)
	if err != nil {
		status := githubclient.StatusOf(err)
		applog.Err.Printf("github fetch event_id=%s repo=%s pr=%d status=%d: %v", msg.EventID, msg.Repo, msg.PRNumber, status, err)
		row.Rationale = "github fetch failed"
		row.Error = err.Error()
		if err := db.Upsert(ctx, row); err != nil {
			return err
		}
		return nil
	}

	row.Title = enr.Title
	if enr.HTMLURL != "" {
		row.PRURL = enr.HTMLURL
	}
	if enr.Author != "" {
		row.Author = enr.Author
	}
	row.Error = ""
	applyClassification(ctx, ollama, &row, enr)

	if err := db.Upsert(ctx, row); err != nil {
		return err
	}
	applog.Info.Printf("upserted event_id=%s repo=%s pr=%d category=%s source=%s title=%q body_len=%d files=%d",
		msg.EventID, msg.Repo, msg.PRNumber, row.Category, row.Source, row.Title, len(enr.Body), len(enr.Files))
	return nil
}

func applyClassification(ctx context.Context, ollama reason.Completer, row *store.Row, enr *githubclient.Enrichment) {
	if strings.TrimSpace(enr.Body) == "" && len(enr.Files) == 0 {
		row.Rationale = "empty body and no files"
		return
	}
	names := make([]string, 0, len(enr.Files))
	for _, f := range enr.Files {
		names = append(names, f.Filename)
	}
	if cat, ok := skipllm.Match(skipllm.Default(), names); ok {
		row.Category = cat
		row.Source = "rule"
		row.Rationale = skipllm.Rationale(cat) + "; " + enrichmentRationale(enr)
		applog.Info.Printf("skip-llm event_id=%s category=%s", row.EventID, cat)
		return
	}

	files := make([]reason.FileInput, 0, len(enr.Files))
	for _, f := range enr.Files {
		files = append(files, reason.FileInput{Filename: f.Filename, Status: f.Status, Patch: f.Patch})
	}
	out := reason.ClassifyPR(ctx, ollama, reason.Input{Title: enr.Title, Body: enr.Body, Files: files}, row.EventID)
	row.Category = out.Category
	row.Source = out.Source
	row.Rationale = out.Rationale
	row.Error = out.Error
	row.Confidence = out.Confidence
	row.AffectedArea = out.AffectedArea
	applog.Info.Printf("llm done event_id=%s category=%s source=%s extract_retries=%d classify_retries=%d",
		row.EventID, out.Category, out.Source, out.ExtractRetries, out.ClassifyRetries)
}

func enrichmentRationale(enr *githubclient.Enrichment) string {
	if strings.TrimSpace(enr.Body) == "" && len(enr.Files) == 0 {
		return "empty body and no files"
	}
	parts := make([]string, 0, len(enr.Files))
	for _, f := range enr.Files {
		parts = append(parts, f.Filename+" ("+f.Status+")")
	}
	return fmt.Sprintf("body_len=%d files=%d: %s", len(enr.Body), len(enr.Files), strings.Join(parts, ", "))
}
