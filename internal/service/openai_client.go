package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type OpenAIClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	cfg        OpenAIConfig
}

type OpenAIConfig struct {
	APIKey     string
	BaseURL    string
	Model      string
	MaxRetries int
	Timeout    time.Duration
}

type EmbeddingRequest struct {
	Input any    `json:"input"`
	Model string `json:"model"`
}

type EmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

type OpenAIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func NewOpenAIClient(cfg OpenAIConfig) (*OpenAIClient, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	httpClient := &http.Client{
		Timeout: cfg.Timeout,
	}

	return &OpenAIClient{
		apiKey:     cfg.APIKey,
		baseURL:    cfg.BaseURL,
		httpClient: httpClient,
		cfg:        cfg,
	}, nil
}

func (c *OpenAIClient) CreateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	batchSize := 10
	var allVectors [][]float32

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		vectors, err := c.processBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to process batch %d-%d: %w", i, end, err)
		}

		allVectors = append(allVectors, vectors...)
		log.Printf("Processed batch %d-%d, total vectors: %d", i, end, len(allVectors))

		if end < len(texts) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return allVectors, nil
}

func (c *OpenAIClient) processBatch(ctx context.Context, texts []string) ([][]float32, error) {
	req := EmbeddingRequest{
		Input: texts,
		Model: c.cfg.Model,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	var resp *EmbeddingResponse
	var err error

	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying OpenAI request, attempt %d/%d", attempt+1, c.cfg.MaxRetries+1)
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		resp, err = c.makeRequest(timeoutCtx, req)
		if err == nil {
			break
		}

		log.Printf("OpenAI request failed (attempt %d): %v", attempt+1, err)
	}

	if err != nil {
		return nil, fmt.Errorf("all retry attempts failed: %w", err)
	}

	vectors := make([][]float32, len(resp.Data))
	for i, embedding := range resp.Data {
		vector := make([]float32, len(embedding.Embedding))
		for j, val := range embedding.Embedding {
			vector[j] = float32(val)
		}
		vectors[i] = vector
	}

	return vectors, nil
}

func (c *OpenAIClient) makeRequest(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/embeddings", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var openAIErr OpenAIError
		if err := json.Unmarshal(body, &openAIErr); err == nil && openAIErr.Error.Message != "" {
			return nil, fmt.Errorf("OpenAI API error: %s (code: %s)", openAIErr.Error.Message, openAIErr.Error.Code)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var embeddingResp EmbeddingResponse
	if err := json.Unmarshal(body, &embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &embeddingResp, nil
}

func (c *OpenAIClient) Close() error {
	return nil
}
