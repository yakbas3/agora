DROP INDEX IF EXISTS idx_endpoints_embedding;
ALTER TABLE endpoints ALTER COLUMN embedding TYPE vector(1536);
