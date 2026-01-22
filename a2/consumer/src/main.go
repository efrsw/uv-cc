package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	uvkafka "univie.ac.at/efrsw/consumer/uv_kafka"
	uvmongo "univie.ac.at/efrsw/consumer/uv_mongo"
	uvserver "univie.ac.at/efrsw/consumer/uv_server"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	messageBus := make(chan uvkafka.LeafUpdateEvent, 200)
	defer close(messageBus)

	kafkaSub := uvkafka.KafkaSubscription{
		BootstrapServer: getEnv("KAFKA_BOOTSTRAP_SERVER", "localhost:9093"),
		GroupId:         getEnv("KAFKA_GROUP_ID", "test-group"),
	}
	if err := kafkaSub.Initialize(
		[]string{"tomato", "pepper", "potato"}); err != nil {
		slog.Error("Failed to initialize Kafka", "error", err)
		os.Exit(1)
	}
	defer kafkaSub.Close()

	mongoConnection := uvmongo.MongoConnection{}
	if err := mongoConnection.Initialize(ctx,
		getEnv("MONGO_URI", "mongodb://localhost:27018")); err != nil {
		slog.Error("Failed to initialize MongoDB", "error", err)
		os.Exit(1)
	}
	defer mongoConnection.Close(context.Background())

	srv := uvserver.LeafServer{
		Port:  getEnv("SERVER_PORT", ":80"),
		Mongo: &mongoConnection,
	}

	var wg sync.WaitGroup

	wg.Go(func() { kafkaSub.ProcessMessagesLoop(ctx, messageBus) })

	wg.Go(func() { mongoConnection.HandleMessage(ctx, messageBus) })

	wg.Go(func() {
		if err := srv.ListenAndServe(ctx); err != nil {
			slog.Error("Server error", "error", err)
		}
	})

	slog.Info("Consumer service started")

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	<-sigChan

	cancel()
	go func() {
		time.Sleep(2 * time.Second)
		close(messageBus)
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("Graceful shutdown complete")
	case <-time.After(30 * time.Second):
		slog.Warn("Shutdown timeout exceeded")
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
