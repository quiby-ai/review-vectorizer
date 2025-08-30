-- Initialize review_embeddings table for the review vectorizer application
-- This script creates the necessary table structure and indexes

-- Enable the pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Create the review_embeddings table
CREATE TABLE IF NOT EXISTS review_embeddings (
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
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_review_embeddings_app_id ON review_embeddings(app_id);
CREATE INDEX IF NOT EXISTS idx_review_embeddings_language ON review_embeddings(language);
CREATE INDEX IF NOT EXISTS idx_review_embeddings_rating ON review_embeddings(rating);
CREATE INDEX IF NOT EXISTS idx_review_embeddings_country ON review_embeddings(country);
CREATE INDEX IF NOT EXISTS idx_review_embeddings_model ON review_embeddings(model);
CREATE INDEX IF NOT EXISTS idx_review_embeddings_created_at ON review_embeddings(created_at);

-- Add migration columns (for future use)
ALTER TABLE review_embeddings ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();
CREATE INDEX IF NOT EXISTS idx_review_embeddings_updated_at ON review_embeddings(updated_at);

-- Verify the table structure
SELECT 
    column_name, 
    data_type, 
    is_nullable,
    column_default
FROM information_schema.columns 
WHERE table_name = 'review_embeddings' 
ORDER BY ordinal_position;
