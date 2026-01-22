package uvkafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type KafkaSubscription struct {
	BootstrapServer string
	GroupId         string
	c               *kafka.Consumer
}

// Initialize new Consumer
func (ks *KafkaSubscription) Initialize(topics []string) error {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  ks.BootstrapServer,
		"group.id":           ks.GroupId,
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": false,
	})

	if err != nil {
		return err
	}

	ks.c = c

	err = c.SubscribeTopics(topics, nil)
	if err != nil {
		return err
	}

	return nil
}

// Close current Consumer
func (ks *KafkaSubscription) Close() error {
	if err := ks.c.Close(); err != nil {
		return err
	}

	ks.c = nil
	return nil
}

// Start reading messages from 3 topics into a channel
func (ks *KafkaSubscription) ProcessMessagesLoop(ctx context.Context, mb chan LeafUpdateEvent) {
	slog.Info("Starting Kafka message processing loop")

	for {
		select {
		case <-ctx.Done():
			slog.Info("Context cancelled, stopping Kafka consumer")
			return
		default:
			msg, err := ks.c.ReadMessage(100 * time.Millisecond)

			if err == nil {
				slog.Info("Received message",
					"topic", *msg.TopicPartition.Topic,
					"partition", msg.TopicPartition.Partition,
					"offset", msg.TopicPartition.Offset)

				var message LeafMessage
				if err := json.Unmarshal(msg.Value, &message); err != nil {
					slog.Error("Failed to unmarshal message",
						"error", err,
						"topic", *msg.TopicPartition.Topic)
					continue
				}

				// Check context before sending to channel
				select {
				case <-ctx.Done():
					slog.Info("Context cancelled before sending message")
					return
				case mb <- LeafUpdateEvent{
					*msg.TopicPartition.Topic,
					message,
					func() { ks.c.CommitMessage(msg) },
				}: // Message sent
				}
			} else {
				kafkaErr, ok := err.(kafka.Error)
				if !ok || !kafkaErr.IsTimeout() {
					// The client will automatically try to recover from all errors.
					// Timeout is not considered an error because it is raised by
					// ReadMessage in absence of messages.
					slog.Error("Consumer error",
						"error", err)
				}
			}
		}
	}
}
