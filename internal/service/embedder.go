package service

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
)

type Embedder interface {
	EmbedBatch(ctx context.Context, inputs []string) ([][]float32, error)
}

type OpenAIEmbedder struct {
	client *OpenAIClient
	logger *slog.Logger
}

func NewOpenAIEmbedder(client *OpenAIClient, logger *slog.Logger) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		client: client,
		logger: logger,
	}
}

func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	processedInputs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		if processed := preprocessText(input); processed != "" {
			processedInputs = append(processedInputs, processed)
		}
	}

	if len(processedInputs) == 0 {
		return nil, fmt.Errorf("no valid inputs after preprocessing")
	}

	e.logger.Debug("Generating embeddings", "count", len(processedInputs))

	vectors, err := e.client.CreateEmbeddings(ctx, processedInputs)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	e.logger.Debug("Generated embeddings successfully", "count", len(vectors))
	return vectors, nil
}

type StubEmbedder struct {
	dim    int
	logger *slog.Logger
}

func NewStubEmbedder(dim int, logger *slog.Logger) *StubEmbedder {
	return &StubEmbedder{
		dim:    dim,
		logger: logger,
	}
}

func (e *StubEmbedder) EmbedBatch(ctx context.Context, inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	e.logger.Debug("Generating stub embeddings", "count", len(inputs), "dim", e.dim)

	vectors := make([][]float32, len(inputs))
	for i := range inputs {
		vector := make([]float32, e.dim)
		for j := range vector {
			vector[j] = float32(rand.Float64() * 0.01)
		}
		vectors[i] = vector
	}

	e.logger.Debug("Generated stub embeddings", "count", len(vectors))
	return vectors, nil
}

func preprocessText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.Join(strings.Fields(text), " ")

	if len(text) < 3 {
		return ""
	}

	return text
}
