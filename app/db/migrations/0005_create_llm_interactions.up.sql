-- +migrate Up
-- Table to log interactions with the LLM (Gemini) for debugging, analysis, history
CREATE TABLE llm_interactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    user_id UUID REFERENCES users (id) ON DELETE SET NULL, -- Link to user if applicable
    session_id TEXT, -- Optional: Group related interactions
    prompt TEXT NOT NULL, -- The final prompt sent
    request_payload JSONB, -- Full request body sent to Gemini API (optional)
    response_text TEXT, -- The final generated text response
    response_payload JSONB, -- Full response body from Gemini API (incl. function calls, safety ratings)
    model_used TEXT, -- e.g., 'gemini-1.5-pro'
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    total_tokens INTEGER,
    latency_ms INTEGER, -- Time taken for the API call
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for querying logs
CREATE INDEX idx_llm_interactions_user_id ON llm_interactions (user_id);

CREATE INDEX idx_llm_interactions_created_at ON llm_interactions (created_at);
-- Consider JSONB indexes if querying payload frequently:
-- CREATE INDEX idx_llm_interactions_resp_payload_gin ON llm_interactions USING GIN (response_payload);


CREATE TABLE llm_suggested_pois (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL, -- The user for whom this was generated
    search_profile_id UUID, -- The specific search profile used (if applicable)
    llm_interaction_id UUID NOT NULL REFERENCES llm_interactions(id) ON DELETE CASCADE, -- Links to the LLM request/response log
    city_id UUID REFERENCES cities(id) ON DELETE SET NULL, -- The city context for this POI

    name TEXT NOT NULL,
    description_poi TEXT, -- LLM-generated description
    location GEOMETRY(Point, 4326) NOT NULL, -- Store LLM provided lat/lon here
    category TEXT, -- LLM-suggested category
    address TEXT, -- If LLM provides it
    website TEXT, -- If LLM provides it
    opening_hours_suggestion TEXT, -- If LLM provides it
    -- You can add other fields from types.POIDetail if the LLM commonly provides them

-- Foreign key constraints (if not defined inline above)
-- CONSTRAINT fk_llm_suggested_pois_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE, -- Assuming you have a users table
-- CONSTRAINT fk_llm_suggested_pois_profile FOREIGN KEY (search_profile_id) REFERENCES user_search_profiles(id) ON DELETE SET NULL,

created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_llm_suggested_pois_user_id ON llm_suggested_pois (user_id);

CREATE INDEX idx_llm_suggested_pois_search_profile_id ON llm_suggested_pois (search_profile_id);

CREATE INDEX idx_llm_suggested_pois_llm_interaction_id ON llm_suggested_pois (llm_interaction_id);

CREATE INDEX idx_llm_suggested_pois_city_id ON llm_suggested_pois (city_id);

CREATE INDEX idx_llm_suggested_pois_location ON llm_suggested_pois USING GIST (location);
-- Crucial for distance sorting

-- Trigger to update 'updated_at' timestamp
CREATE TRIGGER trigger_set_llm_suggested_pois_updated_at
BEFORE UPDATE ON llm_suggested_pois
FOR EACH ROW EXECUTE FUNCTION set_updated_at();