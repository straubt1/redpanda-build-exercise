package kafka

import (
	"context"
	"fmt"

	"github.com/straubt1/redpanda-build-exercise/internal/applog"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Consumer struct {
	client *kgo.Client
}

func NewConsumer(brokers []string, topic, group string) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
		// New group: read existing topic messages (local verify). Committed offsets still win.
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka client: %w", err)
	}
	return &Consumer{client: client}, nil
}

func (c *Consumer) Close() {
	c.client.Close()
}

func (c *Consumer) Poll(ctx context.Context) ([]*kgo.Record, error) {
	fetches := c.client.PollFetches(ctx)
	if errs := fetches.Errors(); len(errs) > 0 {
		for _, e := range errs {
			if e.Err == ctx.Err() {
				return nil, ctx.Err()
			}
			applog.Err.Printf("kafka fetch error topic=%s partition=%d: %v", e.Topic, e.Partition, e.Err)
		}
		if fetches.NumRecords() == 0 {
			return nil, fmt.Errorf("kafka fetch: %w", errs[0].Err)
		}
	}

	var records []*kgo.Record
	iter := fetches.RecordIter()
	for !iter.Done() {
		records = append(records, iter.Next())
	}
	return records, nil
}

func (c *Consumer) Commit(ctx context.Context, rec *kgo.Record) error {
	return c.client.CommitRecords(ctx, rec)
}
