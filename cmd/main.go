package main

import (
	"context"
	"log"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/quiby-ai/review-vectorizer/config"
	"github.com/quiby-ai/review-vectorizer/internal/consumer"
	"github.com/quiby-ai/review-vectorizer/internal/producer"
	"github.com/quiby-ai/review-vectorizer/internal/service"
	"github.com/quiby-ai/review-vectorizer/internal/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(log.Writer(), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("Connecting to database and initializing tables...")
	repo, err := storage.NewPostgresRepository(cfg.Postgres.DSN)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		log.Fatalf("database: %v", err)
	}
	defer repo.Close()

	logger.Info("Database connection established and tables initialized successfully")

	stats, err := repo.GetTableStats(context.Background())
	if err != nil {
		logger.Warn("Failed to get table stats", "error", err)
	} else {
		logger.Info("Table statistics", "stats", stats)
	}

	producer := producer.NewProducer(cfg.Kafka)
	defer producer.Close()

	svc := service.NewVectorizeService(repo, cfg, logger, producer)

	cons := consumer.NewKafkaConsumer(cfg.Kafka, svc)
	if err := cons.Run(ctx); err != nil {
		logger.Error("Consumer exited with error", "error", err)
		log.Fatalf("consumer exited with error: %v", err)
	}
}
