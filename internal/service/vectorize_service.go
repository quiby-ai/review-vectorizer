package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/quiby-ai/common/pkg/events"
	"github.com/quiby-ai/review-vectorizer/config"
	"github.com/quiby-ai/review-vectorizer/internal/producer"
	"github.com/quiby-ai/review-vectorizer/internal/storage"
)

type VectorizeRequest struct {
	ForceRecompute bool
	Limit          int
	AppID          string
	Countries      []string
	Languages      []string
	DateFrom       string
	DateTo         string
}

type VectorizeResult struct {
	Processed int      `json:"processed"`
	Skipped   int      `json:"skipped"`
	Failed    int      `json:"failed"`
	ReviewIDs []string `json:"review_ids"`
}

type VectorizeService struct {
	repo     storage.Repository
	embedder Embedder
	cfg      *config.Config
	logger   *slog.Logger
	producer *producer.Producer
}

func NewVectorizeService(repo storage.Repository, cfg *config.Config, logger *slog.Logger, producer *producer.Producer) *VectorizeService {
	var embedder Embedder

	if cfg.OpenAI.APIKey != "" {
		openAIClient, err := NewOpenAIClient(OpenAIConfig{
			APIKey:     cfg.OpenAI.APIKey,
			BaseURL:    cfg.OpenAI.BaseURL,
			Model:      cfg.OpenAI.Model,
			MaxRetries: cfg.OpenAI.MaxRetries,
			Timeout:    cfg.OpenAI.Timeout,
		})
		if err != nil {
			logger.Warn("Failed to initialize OpenAI client, falling back to stub", "error", err)
			embedder = NewStubEmbedder(cfg.Vectorizer.MaxVectorLength, logger)
		} else {
			embedder = NewOpenAIEmbedder(openAIClient, logger)
		}
	} else {
		logger.Info("No OpenAI API key provided, using stub embedder")
		embedder = NewStubEmbedder(cfg.Vectorizer.MaxVectorLength, logger)
	}

	return &VectorizeService{
		repo:     repo,
		embedder: embedder,
		cfg:      cfg,
		logger:   logger,
		producer: producer,
	}
}

func (s *VectorizeService) RunOnce(ctx context.Context, req VectorizeRequest) (VectorizeResult, error) {
	startTime := time.Now()

	batchSize := s.determineBatchSize(req.Limit)

	s.logger.Info("Starting vectorization run",
		"batch_size", batchSize,
		"force_recompute", req.ForceRecompute,
		"model", s.cfg.Vectorizer.Model,
		"dim", s.cfg.Vectorizer.MaxVectorLength)

	result, err := s.processAllReviews(ctx, req, batchSize)
	if err != nil {
		return VectorizeResult{}, fmt.Errorf("failed to process reviews: %w", err)
	}

	duration := time.Since(startTime)
	s.logger.Info("Vectorization run completed",
		"duration", duration,
		"processed", result.Processed,
		"skipped", result.Skipped,
		"failed", result.Failed)

	return result, nil
}

func (s *VectorizeService) determineBatchSize(limit int) int {
	if limit > 0 {
		return limit
	}
	return s.cfg.Vectorizer.BatchSize
}

func (s *VectorizeService) processAllReviews(ctx context.Context, req VectorizeRequest, batchSize int) (VectorizeResult, error) {
	result := VectorizeResult{}
	offset := 0
	totalProcessed := 0

	filters := storage.CleanReviewFilters{
		ForceRecompute: req.ForceRecompute,
		AppID:          req.AppID,
		Countries:      req.Countries,
		Languages:      req.Languages,
		DateFrom:       req.DateFrom,
		DateTo:         req.DateTo,
	}

	for {
		reviews, err := s.repo.GetCleanReviewsForVectorization(ctx, filters, batchSize, offset)
		if err != nil {
			return result, fmt.Errorf("failed to fetch reviews batch at offset %d: %w", offset, err)
		}

		if len(reviews) == 0 {
			s.logger.Info("No more reviews to process", "total_processed", totalProcessed)
			break
		}

		s.logger.Info("Processing batch of reviews",
			"batch_size", len(reviews),
			"offset", offset,
			"total_processed", totalProcessed)

		batchResult := s.processReviewsInBatches(ctx, reviews)

		result.Processed += batchResult.Processed
		result.Skipped += batchResult.Skipped
		result.Failed += batchResult.Failed
		result.ReviewIDs = append(result.ReviewIDs, batchResult.ReviewIDs...)

		totalProcessed += len(reviews)

		if len(reviews) < batchSize {
			s.logger.Info("Reached end of reviews", "total_processed", totalProcessed)
			break
		}

		offset += batchSize

		select {
		case <-ctx.Done():
			s.logger.Info("Context cancelled, stopping review processing", "total_processed", totalProcessed)
			return result, ctx.Err()
		default:
		}
	}

	return result, nil
}

func (s *VectorizeService) processReviewsInBatches(ctx context.Context, reviews []storage.CleanReview) VectorizeResult {
	result := VectorizeResult{}
	batchSize := s.cfg.Vectorizer.BatchSize

	for i := 0; i < len(reviews); i += batchSize {
		end := min(i+batchSize, len(reviews))

		batch := reviews[i:end]
		batchResult, err := s.processBatch(ctx, batch)
		if err != nil {
			s.logger.Error("Failed to process batch", "batch_start", i, "batch_end", end, "error", err)
			result.Failed += len(batch)
			continue
		}

		result.Processed += batchResult.Processed
		result.Skipped += batchResult.Skipped
		result.Failed += batchResult.Failed
		result.ReviewIDs = append(result.ReviewIDs, batchResult.ReviewIDs...)
	}

	return result
}

func (s *VectorizeService) processBatch(ctx context.Context, reviews []storage.CleanReview) (VectorizeResult, error) {
	if len(reviews) == 0 {
		return VectorizeResult{}, nil
	}

	batchStart := time.Now()
	s.logger.Debug("Processing batch", "count", len(reviews))

	contentTexts, responseTexts := s.prepareTexts(reviews)

	if len(contentTexts) == 0 {
		s.logger.Debug("No valid content texts in batch")
		return VectorizeResult{}, nil
	}

	contentVectors, responseVectors, err := s.generateEmbeddings(ctx, contentTexts, responseTexts)
	if err != nil {
		return VectorizeResult{}, err
	}

	result := s.storeVectors(ctx, reviews, contentVectors, responseVectors)

	batchDuration := time.Since(batchStart)
	s.logger.Debug("Batch processed",
		"count", len(reviews),
		"duration", batchDuration,
		"processed", result.Processed,
		"failed", result.Failed)

	return result, nil
}

func (s *VectorizeService) prepareTexts(reviews []storage.CleanReview) ([]string, []string) {
	contentTexts := make([]string, 0, len(reviews))
	responseTexts := make([]string, 0, len(reviews))

	for _, review := range reviews {
		contentTexts = append(contentTexts, review.ContentClean)

		if review.ResponseContentClean != nil && *review.ResponseContentClean != "" {
			responseTexts = append(responseTexts, *review.ResponseContentClean)
		} else {
			responseTexts = append(responseTexts, "")
		}
	}

	return contentTexts, responseTexts
}

func (s *VectorizeService) generateEmbeddings(ctx context.Context, contentTexts, responseTexts []string) ([][]float32, [][]float32, error) {
	contentVectors, err := s.embedder.EmbedBatch(ctx, contentTexts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate content embeddings: %w", err)
	}

	var responseVectors [][]float32
	nonEmptyResponses := s.filterNonEmptyResponses(responseTexts)

	if len(nonEmptyResponses) > 0 {
		responseVectors, err = s.embedder.EmbedBatch(ctx, nonEmptyResponses)
		if err != nil {
			s.logger.Warn("Failed to generate response embeddings, continuing without them", "error", err)
			responseVectors = nil
		}
	}

	return contentVectors, responseVectors, nil
}

func (s *VectorizeService) filterNonEmptyResponses(responseTexts []string) []string {
	nonEmpty := make([]string, 0)
	for _, text := range responseTexts {
		if text != "" {
			nonEmpty = append(nonEmpty, text)
		}
	}
	return nonEmpty
}

func (s *VectorizeService) storeVectors(ctx context.Context, reviews []storage.CleanReview, contentVectors, responseVectors [][]float32) VectorizeResult {
	result := VectorizeResult{}

	for i, review := range reviews {
		vector := s.createVector(review, contentVectors[i], responseVectors, i)

		if err := s.repo.UpsertEmbedding(ctx, vector); err != nil {
			s.logger.Error("Failed to store embedding", "review_id", review.ID, "error", err)
			result.Failed++
		} else {
			result.Processed++
			result.ReviewIDs = append(result.ReviewIDs, review.ID)
		}
	}

	return result
}

func (s *VectorizeService) createVector(review storage.CleanReview, contentVec []float32, responseVectors [][]float32, index int) *storage.Vector {
	vector := storage.NewVector(review.ID, review.AppID, contentVec)

	vector.Language = review.Language
	vector.Rating = review.Rating
	vector.Country = review.Country
	vector.Model = s.cfg.Vectorizer.Model
	vector.Dim = s.cfg.Vectorizer.MaxVectorLength
	vector.CreatedAt = time.Now()

	if responseVectors != nil && index < len(responseVectors) {
		vector.ResponseVec = responseVectors[index]
	}

	return vector
}

func (s *VectorizeService) Handle(ctx context.Context, payload any, sagaID string) error {
	s.logger.Info("Processing vectorization event", "saga_id", sagaID, "payload_type", fmt.Sprintf("%T", payload))

	req := s.extractRequestFromPayload(payload)

	s.logger.Info("Vectorization request",
		"force_recompute", req.ForceRecompute,
		"limit", req.Limit,
		"app_id", req.AppID,
		"countries", req.Countries,
		"languages", req.Languages,
		"date_from", req.DateFrom,
		"date_to", req.DateTo,
		"saga_id", sagaID)

	result, err := s.RunOnce(ctx, req)
	if err != nil {
		s.logger.Error("Vectorization failed", "error", err, "saga_id", sagaID)
		return fmt.Errorf("vectorization failed: %w", err)
	}

	s.logger.Info("Vectorization completed successfully",
		"processed", result.Processed,
		"skipped", result.Skipped,
		"failed", result.Failed,
		"saga_id", sagaID)

	if err = s.publishCompletedEvent(ctx, payload, sagaID); err != nil {
		s.logger.Error("Failed to publish completed event", "error", err, "saga_id", sagaID)
	}

	return nil
}

func (s *VectorizeService) extractRequestFromPayload(payload any) VectorizeRequest {
	var req VectorizeRequest

	switch p := payload.(type) {
	case map[string]any:
		if force, ok := p["force_recompute"].(bool); ok {
			req.ForceRecompute = force
		}
		if limit, ok := p["limit"].(float64); ok {
			req.Limit = int(limit)
		}
		if batchSize, ok := p["batch_size"].(float64); ok {
			req.Limit = int(batchSize)
		}
		if appID, ok := p["app_id"].(string); ok {
			req.AppID = appID
		}
		if countries, ok := p["countries"].([]any); ok {
			req.Countries = make([]string, len(countries))
			for i, country := range countries {
				if countryStr, ok := country.(string); ok {
					req.Countries[i] = countryStr
				}
			}
		}
		if languages, ok := p["languages"].([]any); ok {
			req.Languages = make([]string, len(languages))
			for i, language := range languages {
				if languageStr, ok := language.(string); ok {
					req.Languages[i] = languageStr
				}
			}
		}
		if dateFrom, ok := p["date_from"].(string); ok {
			req.DateFrom = dateFrom
		}
		if dateTo, ok := p["date_to"].(string); ok {
			req.DateTo = dateTo
		}
	case string:
		if p == "force" || p == "recompute" {
			req.ForceRecompute = true
		}
	default:
		s.logger.Info("Unknown payload type, using default vectorization request")
	}

	return req
}

func (s *VectorizeService) publishCompletedEvent(ctx context.Context, payload any, sagaID string) error {
	evt := payload.(events.VectorizeRequest)

	completedEvent := events.VectorizeCompleted{
		VectorizeRequest: evt,
	}

	envelope := s.producer.BuildEnvelope(completedEvent, sagaID)
	return s.producer.PublishEvent(ctx, []byte(sagaID), envelope)
}
