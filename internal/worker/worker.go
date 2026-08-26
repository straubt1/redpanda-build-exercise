package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/straubt1/redpanda-build-exercise/internal/applog"
	"github.com/straubt1/redpanda-build-exercise/internal/config"
	"github.com/straubt1/redpanda-build-exercise/internal/kafka"
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

	applog.Info.Printf("consuming topic=%s group=%s brokers=%v", cfg.KafkaTopic, cfg.KafkaGroup, cfg.KafkaBrokers)

	for {
		records, err := c.Poll(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		for _, rec := range records {
			if err := handle(ctx, db, rec.Value); err != nil {
				return fmt.Errorf("upsert offset=%d: %w", rec.Offset, err)
			}
			if err := c.Commit(ctx, rec); err != nil {
				return fmt.Errorf("commit offset=%d: %w", rec.Offset, err)
			}
		}
	}
}

func handle(ctx context.Context, db *store.Store, raw []byte) error {
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

	if err := db.Upsert(ctx, row); err != nil {
		return err
	}
	applog.Info.Printf("upserted event_id=%s repo=%s pr=%d", msg.EventID, msg.Repo, msg.PRNumber)
	return nil
}
