package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/straubt1/redpanda-build-exercise/internal/applog"
	"github.com/straubt1/redpanda-build-exercise/internal/config"
	"github.com/straubt1/redpanda-build-exercise/internal/githubclient"
	"github.com/straubt1/redpanda-build-exercise/internal/kafka"
	"github.com/straubt1/redpanda-build-exercise/internal/llm"
	"github.com/straubt1/redpanda-build-exercise/internal/reason"
	"github.com/straubt1/redpanda-build-exercise/internal/store"
)

// Message is the Connect-projected topic payload.
type Message struct {
	EventID   string `json:"event_id"`
	Repo      string `json:"repo"`
	PRNumber  int    `json:"pr_number"`
	PRURL     string `json:"pr_url"`
	Author    string `json:"author"`
	Action    string `json:"action"`
	CreatedAt string `json:"created_at"`
}

func Run(ctx context.Context, cfg config.Config) error {
	// Connect to the database
	db, err := store.Connect(ctx, cfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	// Connect to the Kafka
	c, err := kafka.NewConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroup)
	if err != nil {
		return err
	}
	defer c.Close()

	// Connect to the Github API
	gh := githubclient.New(cfg.GitHubToken, cfg.GitHubUserAgent)
	ollama := llm.New(cfg.OllamaURL, cfg.OllamaModel)

	applog.Info.Printf("consuming topic=%s group=%s brokers=%v ollama=%s model=%s",
		cfg.KafkaTopic, cfg.KafkaGroup, cfg.KafkaBrokers, cfg.OllamaURL, cfg.OllamaModel)

	for { // Loop indefinitely, polling for new messages from Kafka
		records, err := c.Poll(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		for _, rec := range records {
			// Handle the message and upsert the DB - the main logic of the worker
			if err := handleMessage(ctx, db, gh, ollama, rec.Value); err != nil {
				return fmt.Errorf("upsert offset=%d: %w", rec.Offset, err)
			}
			// After a successful upsert: tell the consumer group this offset is done.
			// Auto-commit is off, so a crash before this line redelivers the same message.
			if err := c.Commit(ctx, rec); err != nil {
				return fmt.Errorf("commit offset=%d: %w", rec.Offset, err)
			}
		}
	}
}

func handleMessage(ctx context.Context, db *store.Store, gh *githubclient.Client, ollama reason.Completer, raw []byte) error {
	// Parse the message from Kafka and Validate the data
	msg, ok := parseMessage(raw)
	if !ok {
		return nil
	}

	// Enrich the data with GH API by fetching the PR data not in the kafka message
	enr, err := enrich(ctx, gh, msg)
	if err != nil {
		return persistFetchFailure(ctx, db, msg, err)
	}

	// Apply the classification - this the top level classification of the PR after all needed data is fetched
	out := reason.Classify(ctx, ollama, inputFrom(enr), msg.EventID)
	return persist(ctx, db, msg, enr, out)
}

// Parse the message from Kafka and Validate the data
func parseMessage(raw []byte) (Message, bool) {
	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		applog.Err.Printf("skip bad json: %v", err)
		return Message{}, false
	}
	if msg.EventID == "" || msg.Repo == "" || msg.PRNumber == 0 {
		applog.Err.Printf("skip incomplete message event_id=%q repo=%q pr=%d", msg.EventID, msg.Repo, msg.PRNumber)
		return Message{}, false
	}
	return msg, true
}

// Enrich the data with GH API by fetching the PR data not in the kafka message
func enrich(ctx context.Context, gh *githubclient.Client, msg Message) (*githubclient.Enrichment, error) {
	return gh.Fetch(ctx, msg.Repo, msg.PRNumber)
}

func inputFrom(enr *githubclient.Enrichment) reason.Input {
	files := make([]reason.FileInput, 0, len(enr.Files))
	for _, f := range enr.Files {
		files = append(files, reason.FileInput{
			Filename:  f.Filename,
			Status:    f.Status,
			Patch:     f.Patch,
			Additions: f.Additions,
			Deletions: f.Deletions,
			Changes:   f.Changes,
		})
	}
	return reason.Input{Title: enr.Title, Body: enr.Body, Files: files}
}

func persist(ctx context.Context, db *store.Store, msg Message, enr *githubclient.Enrichment, out reason.Outcome) error {
	row := rowFromMessage(msg)
	row.Title = enr.Title
	if enr.HTMLURL != "" {
		row.PRURL = enr.HTMLURL
	}
	if enr.Author != "" {
		row.Author = enr.Author
	}
	row.Category = out.Category
	row.Source = out.Source
	row.Rationale = out.Rationale
	row.Error = out.Error
	row.Confidence = out.Confidence
	row.AffectedArea = out.AffectedArea
	row.Summary = out.Summary
	if err := db.Upsert(ctx, row); err != nil {
		return err
	}
	applog.Info.Printf("upserted event_id=%s repo=%s pr=%d category=%s source=%s title=%q body_len=%d files=%d summarize_retries=%d classify_retries=%d",
		msg.EventID, msg.Repo, msg.PRNumber, row.Category, row.Source, row.Title, len(enr.Body), len(enr.Files), out.SummarizeRetries, out.ClassifyRetries)
	return nil
}

func persistFetchFailure(ctx context.Context, db *store.Store, msg Message, fetchErr error) error {
	status := githubclient.StatusOf(fetchErr)
	applog.Err.Printf("github fetch event_id=%s repo=%s pr=%d status=%d: %v", msg.EventID, msg.Repo, msg.PRNumber, status, fetchErr)
	row := rowFromMessage(msg)
	row.Rationale = "github fetch failed"
	row.Error = fetchErr.Error()
	if err := db.Upsert(ctx, row); err != nil {
		return err
	}
	return nil
}

func rowFromMessage(msg Message) store.Row {
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
		row.CreatedAt = &t
	}
	return row
}
