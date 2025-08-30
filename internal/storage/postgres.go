package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

type Repository interface {
	GetCleanReviewsForVectorization(ctx context.Context, forceRecompute bool, limit int) ([]CleanReview, error)
	UpsertEmbedding(ctx context.Context, vector *Vector) error
	GetTableStats(ctx context.Context) (map[string]any, error)
	Close() error
}

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(dsn string) (Repository, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &postgresRepository{db: pool}

	if err := repo.initTables(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return repo, nil
}

func (r *postgresRepository) initTables(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS review_embeddings (
			embedding_id VARCHAR(255) PRIMARY KEY,
			review_id VARCHAR(255) UNIQUE NOT NULL,
			app_id VARCHAR(255) NOT NULL,
			language VARCHAR(10),
			rating SMALLINT,
			country VARCHAR(10),
			model VARCHAR(100) NOT NULL,
			dim INTEGER NOT NULL,
			content_vec vector(1536),
			response_vec vector(1536),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_review_embeddings_app_id ON review_embeddings(app_id);`,
		`CREATE INDEX IF NOT EXISTS idx_review_embeddings_language ON review_embeddings(language);`,
		`CREATE INDEX IF NOT EXISTS idx_review_embeddings_rating ON review_embeddings(rating);`,
		`CREATE INDEX IF NOT EXISTS idx_review_embeddings_country ON review_embeddings(country);`,
		`CREATE INDEX IF NOT EXISTS idx_review_embeddings_model ON review_embeddings(model);`,
		`CREATE INDEX IF NOT EXISTS idx_review_embeddings_created_at ON review_embeddings(created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_review_embeddings_updated_at ON review_embeddings(updated_at);`,
	}

	for i, query := range queries {
		if _, err := r.db.Exec(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query %d: %w", i+1, err)
		}
	}

	return nil
}

func (r *postgresRepository) GetTableStats(ctx context.Context) (map[string]any, error) {
	query := `
		SELECT 
			COUNT(*) as total_embeddings,
			COUNT(DISTINCT app_id) as unique_apps,
			COUNT(DISTINCT language) as unique_languages,
			COUNT(DISTINCT model) as unique_models,
			AVG(dim) as avg_dimension,
			MIN(created_at) as oldest_embedding,
			MAX(created_at) as newest_embedding
		FROM review_embeddings;
	`

	row := r.db.QueryRow(ctx, query)

	var totalEmbeddings, uniqueApps, uniqueLanguages, uniqueModels int64
	var avgDimension float64
	var oldestEmbedding, newestEmbedding string

	if err := row.Scan(
		&totalEmbeddings,
		&uniqueApps,
		&uniqueLanguages,
		&uniqueModels,
		&avgDimension,
		&oldestEmbedding,
		&newestEmbedding,
	); err != nil {
		return nil, fmt.Errorf("failed to scan table stats: %w", err)
	}

	stats := map[string]any{
		"total_embeddings": totalEmbeddings,
		"unique_apps":      uniqueApps,
		"unique_languages": uniqueLanguages,
		"unique_models":    uniqueModels,
		"avg_dimension":    avgDimension,
		"oldest_embedding": oldestEmbedding,
		"newest_embedding": newestEmbedding,
	}

	return stats, nil
}

func (r *postgresRepository) GetCleanReviewsForVectorization(ctx context.Context, forceRecompute bool, limit int) ([]CleanReview, error) {
	query := `
		SELECT
			cr.id, cr.app_id, cr.country, cr.rating, cr.language,
			cr.content_clean, cr.content_en, cr.response_content_clean
		FROM clean_reviews cr
		LEFT JOIN review_embeddings re ON re.review_id = cr.id
		WHERE cr.is_contentful = true
			AND cr.content_clean IS NOT NULL
			AND (re.review_id IS NULL OR $1::bool = true)
		ORDER BY cr.reviewed_at DESC
		LIMIT $2;
	`

	rows, err := r.db.Query(ctx, query, forceRecompute, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query clean reviews: %w", err)
	}
	defer rows.Close()

	var reviews []CleanReview
	for rows.Next() {
		var review CleanReview
		if err := rows.Scan(
			&review.ID,
			&review.AppID,
			&review.Country,
			&review.Rating,
			&review.Language,
			&review.ContentClean,
			&review.ContentEN,
			&review.ResponseContentClean,
		); err != nil {
			return nil, fmt.Errorf("failed to scan review: %w", err)
		}
		reviews = append(reviews, review)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return reviews, nil
}

func (r *postgresRepository) UpsertEmbedding(ctx context.Context, vector *Vector) error {
	query := `
		INSERT INTO review_embeddings
			(embedding_id, review_id, app_id, language, rating, country, model, dim, content_vec, response_vec)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (review_id) DO NOTHING;
	`

	contentVec := pgvector.NewVector(vector.ContentVec)
	var responseVec *pgvector.Vector
	if len(vector.ResponseVec) > 0 {
		vec := pgvector.NewVector(vector.ResponseVec)
		responseVec = &vec
	}

	_, err := r.db.Exec(ctx, query,
		vector.EmbeddingID,
		vector.ReviewID,
		vector.AppID,
		vector.Language,
		vector.Rating,
		vector.Country,
		vector.Model,
		vector.Dim,
		contentVec,
		responseVec,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert embedding for review %s: %w", vector.ReviewID, err)
	}

	return nil
}

func (r *postgresRepository) Close() error {
	r.db.Close()
	return nil
}
