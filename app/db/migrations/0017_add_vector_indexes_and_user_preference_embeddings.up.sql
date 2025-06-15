-- Add vector indexes for pgvector performance optimization
-- HNSW (Hierarchical Navigable Small World) indexes for efficient similarity search

-- Create HNSW index on points_of_interest embedding column
-- Using cosine distance as it's most suitable for normalized embeddings
CREATE INDEX IF NOT EXISTS idx_poi_embedding_hnsw 
ON points_of_interest 
USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);

-- Create HNSW index on cities embedding column
CREATE INDEX IF NOT EXISTS idx_cities_embedding_hnsw 
ON cities 
USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);

-- Add user preference embedding column to user_interests table (junction table)
ALTER TABLE user_interests 
ADD COLUMN IF NOT EXISTS preference_embedding VECTOR(768);

-- Create HNSW index on user_interests embedding column
CREATE INDEX IF NOT EXISTS idx_user_interests_embedding_hnsw 
ON user_interests 
USING hnsw (preference_embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);

-- Create a function to calculate cosine similarity
-- This will be useful for hybrid queries combining multiple similarity measures
CREATE OR REPLACE FUNCTION cosine_similarity(a vector, b vector) 
RETURNS float AS $$
BEGIN
    RETURN 1 - (a <=> b);
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Create a function to calculate euclidean distance
CREATE OR REPLACE FUNCTION euclidean_distance(a vector, b vector) 
RETURNS float AS $$
BEGIN
    RETURN a <-> b;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Add an index on points_of_interest for faster category searches combined with embeddings
CREATE INDEX IF NOT EXISTS idx_poi_category_embedding 
ON points_of_interest (poi_type, embedding)
WHERE embedding IS NOT NULL;

-- Add an index on cities for faster country-based searches combined with embeddings  
CREATE INDEX IF NOT EXISTS idx_cities_country_embedding 
ON cities (country, embedding)
WHERE embedding IS NOT NULL;

-- Update points_of_interest table to track if embedding has been generated
ALTER TABLE points_of_interest 
ADD COLUMN IF NOT EXISTS embedding_generated_at TIMESTAMP WITH TIME ZONE;

-- Update cities table to track if embedding has been generated
ALTER TABLE cities 
ADD COLUMN IF NOT EXISTS embedding_generated_at TIMESTAMP WITH TIME ZONE;

-- Add comment to explain embedding dimensions
COMMENT ON COLUMN points_of_interest.embedding IS 'Text embedding vector (768 dimensions) generated using Gemini text-embedding-004 model';
COMMENT ON COLUMN cities.embedding IS 'Text embedding vector (768 dimensions) generated using Gemini text-embedding-004 model';
COMMENT ON COLUMN user_interests.preference_embedding IS 'User preference embedding vector (768 dimensions) based on interests and preferences';

-- Create a partial index for POIs that have embeddings (for faster semantic searches)
CREATE INDEX IF NOT EXISTS idx_poi_with_embeddings 
ON points_of_interest (id, name, poi_type)
WHERE embedding IS NOT NULL;

-- Create a partial index for cities that have embeddings
CREATE INDEX IF NOT EXISTS idx_cities_with_embeddings 
ON cities (id, name, country)
WHERE embedding IS NOT NULL;