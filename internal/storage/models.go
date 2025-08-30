package storage

import (
	"time"

	"github.com/google/uuid"
)

type CleanReview struct {
	ID                   string     `json:"id"`
	AppID                string     `json:"app_id"`
	Country              string     `json:"country"`
	Rating               int16      `json:"rating"`
	Title                string     `json:"title"`
	ContentClean         string     `json:"content_clean"`
	Language             string     `json:"language"`
	ContentEN            *string    `json:"content_en"`
	IsContentful         bool       `json:"is_contentful"`
	ReviewedAt           time.Time  `json:"reviewed_at"`
	ResponseDate         *time.Time `json:"response_date"`
	ResponseContentClean *string    `json:"response_content_clean"`
}

type Vector struct {
	EmbeddingID string    `json:"embedding_id"`
	ReviewID    string    `json:"review_id"`
	AppID       string    `json:"app_id"`
	Language    string    `json:"language"`
	Rating      int16     `json:"rating"`
	Country     string    `json:"country"`
	Model       string    `json:"model"`
	Dim         int       `json:"dim"`
	ContentVec  []float32 `json:"content_vec"`
	ResponseVec []float32 `json:"response_vec,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func NewVector(reviewID, appID string, contentVec []float32) *Vector {
	return &Vector{
		EmbeddingID: uuid.New().String(),
		ReviewID:    reviewID,
		AppID:       appID,
		Model:       "text-embedding-3-small",
		Dim:         1536,
		ContentVec:  contentVec,
		CreatedAt:   time.Now(),
	}
}
