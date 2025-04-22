-- +migrate Up
-- Table to log interactions with the LLM (Gemini) for debugging, analysis, history
CREATE TABLE llm_interactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL, -- Link to user if applicable
    session_id TEXT,                                      -- Optional: Group related interactions
    prompt TEXT NOT NULL,                                 -- The final prompt sent
    request_payload JSONB,                                -- Full request body sent to Gemini API (optional)
    response_text TEXT,                                   -- The final generated text response
    response_payload JSONB,                               -- Full response body from Gemini API (incl. function calls, safety ratings)
    model_used TEXT,                                      -- e.g., 'gemini-1.5-pro'
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    total_tokens INTEGER,
    latency_ms INTEGER,                                   -- Time taken for the API call
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for querying logs
CREATE INDEX idx_llm_interactions_user_id ON llm_interactions (user_id);
CREATE INDEX idx_llm_interactions_created_at ON llm_interactions (created_at);
-- Consider JSONB indexes if querying payload frequently:
-- CREATE INDEX idx_llm_interactions_resp_payload_gin ON llm_interactions USING GIN (response_payload);