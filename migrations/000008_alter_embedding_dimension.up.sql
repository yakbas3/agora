-- Alter embedding column from vector(1536) to vector(384) for all-MiniLM-L6-v2
ALTER TABLE endpoints ALTER COLUMN embedding TYPE vector(384);

-- Add HNSW index for fast cosine similarity search
CREATE INDEX idx_endpoints_embedding ON endpoints USING hnsw (embedding vector_cosine_ops);
