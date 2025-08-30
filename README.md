# Review Vectorizer

A microservice in the Quiby app ecosystem that converts app reviews into vector embeddings for semantic search and analysis.

## What It Does

The Review Vectorizer takes app reviews from the database and converts them into high-dimensional vectors (embeddings) using OpenAI's text-embedding-3-small model. These vectors enable:

- **Semantic Search**: Find reviews with similar meaning, not just exact text matches
- **Review Analysis**: Group and analyze reviews by sentiment and content
- **Recommendation Systems**: Power AI-driven app recommendations
- **Content Understanding**: Extract insights from review text automatically

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Kafka Topic   │───▶│  Review Vectorizer │───▶│  PostgreSQL +   │
│ VectorizeRequest│    │                  │    │   pgvector      │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

- **Input**: Kafka messages requesting review vectorization
- **Processing**: OpenAI API for embedding generation
- **Storage**: PostgreSQL with pgvector extension for vector operations
- **Output**: Vector embeddings stored in database

## Quick Start

### Prerequisites

- PostgreSQL 12+ with pgvector extension
- OpenAI API key (optional - falls back to stub mode)

### Environment Variables

```bash
PG_DSN="postgres://user:pass@localhost:5432/quiby_db"
OPENAI_API_KEY="your-openai-key"  # Optional
```

### Run

```bash
# Build
make build

# Start
./bin/review-vectorizer
```

## How It Works

1. **Receives Request**: Listens for vectorization requests via Kafka
2. **Fetches Reviews**: Gets clean reviews from `clean_reviews` table
3. **Generates Embeddings**: Uses OpenAI API to create 1536-dimensional vectors
4. **Stores Vectors**: Saves embeddings in `review_embeddings` table
5. **Handles Errors**: Graceful fallback to stub mode if OpenAI unavailable

## Database

The service automatically creates and manages the `review_embeddings` table:

```sql
CREATE TABLE review_embeddings (
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
);
```

## API Usage

Send Kafka messages to trigger vectorization:

```json
{
  "force_recompute": false,
  "limit": 100
}
```

## Development

```bash
# Run tests
make test

# Run with stub embedder (no OpenAI key needed)
OPENAI_API_KEY="" ./bin/review-vectorizer
```

## Integration

This microservice integrates with other Quiby services:

- **Review Service**: Receives clean reviews for processing
- **Search Service**: Provides vector embeddings for semantic search
- **Analytics Service**: Enables review clustering and analysis
- **Recommendation Engine**: Powers content-based recommendations

## Monitoring

The service logs:
- Vectorization progress and statistics
- Database connection status
- OpenAI API usage and errors
- Processing batch results

## Scaling

- **Horizontal**: Run multiple instances with Kafka consumer groups
- **Vertical**: Adjust batch sizes and timeouts via configuration
- **Performance**: Optimized with database indexes and vector operations
