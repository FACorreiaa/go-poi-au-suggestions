-- +migrate Up

CREATE TABLE user_saved_itineraries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    source_llm_interaction_id UUID NULL REFERENCES llm_interactions (id) ON DELETE SET NULL,
    primary_city_id UUID NULL REFERENCES cities (id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    description TEXT NULL,
    markdown_content TEXT NOT NULL,
    tags TEXT [] NULL,
    estimated_duration_days INTEGER NULL,
    estimated_cost_level INTEGER NULL CHECK (
        estimated_cost_level >= 1
        AND estimated_cost_level <= 4
    ),
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    FOREIGN KEY (source_llm_interaction_id) REFERENCES llm_interactions (id) ON DELETE SET NULL,
    FOREIGN KEY (primary_city_id) REFERENCES cities (id) ON DELETE CASCADE
    -- Consider uniqueness constraint if needed, e.g., UNIQUE (user_id, title)
);
-- Indexes for user_saved_itineraries
CREATE INDEX idx_user_saved_itineraries_user_id ON user_saved_itineraries (user_id);

CREATE INDEX idx_user_saved_itineraries_source_llm_interaction_id ON user_saved_itineraries (source_llm_interaction_id);

CREATE INDEX idx_user_saved_itineraries_primary_city_id ON user_saved_itineraries (primary_city_id);

CREATE INDEX idx_user_saved_itineraries_is_public ON user_saved_itineraries (is_public);

CREATE INDEX idx_user_saved_itineraries_tags ON user_saved_itineraries USING GIN (tags);

CREATE TRIGGER trigger_set_user_saved_itineraries_updated_at
BEFORE UPDATE ON user_saved_itineraries
FOR EACH ROW EXECUTE FUNCTION set_updated_at();