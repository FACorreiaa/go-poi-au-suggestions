-- +migrate Up

-- Optional: Create ENUM types for settings with fixed options
CREATE TYPE day_preference_enum AS ENUM (
    'any',      -- No specific preference
    'day',      -- Primarily daytime activities (e.g., 8am - 6pm)
    'night'     -- Primarily evening/night activities (e.g., 6pm - 2am)
    );

CREATE TYPE search_pace_enum AS ENUM (
    'any',      -- No preference
    'relaxed',  -- Fewer, longer activities
    'moderate', -- Standard pace
    'fast'      -- Pack in many activities
    );

-- Table to store user's default settings/preferences
CREATE TABLE user_settings (
                               user_id UUID PRIMARY KEY NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- Default search radius in kilometers (or miles, be consistent)
                               default_search_radius_km NUMERIC(5, 1) DEFAULT 5.0 CHECK (default_search_radius_km > 0),
    -- Preferred time of day for activities
                               preferred_time day_preference_enum DEFAULT 'any',
    -- Preferred budget level (maps to POI price_level, 1-4, 0=any)
                               default_budget_level INTEGER DEFAULT 0 CHECK (default_budget_level >= 0 AND default_budget_level <= 4),
    -- Preferred pace for itineraries/suggestions
                               preferred_pace search_pace_enum DEFAULT 'any',
    -- Other potential flags or preferences
                               prefer_accessible_pois BOOLEAN DEFAULT FALSE, -- Default preference for accessibility info
                               prefer_outdoor_seating BOOLEAN DEFAULT FALSE, -- Example preference
                               prefer_dog_friendly    BOOLEAN DEFAULT FALSE, -- Example preference
    -- Foreign key ensures this record is tied to a valid user
                               created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                               updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Trigger to update 'updated_at' timestamp
CREATE TRIGGER trigger_set_user_settings_updated_at
    BEFORE UPDATE ON user_settings
               FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Automatically create default settings when a new user is created
-- This requires a trigger function and a trigger on the users table.
CREATE OR REPLACE FUNCTION create_default_user_settings()
    RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO user_settings (user_id) VALUES (NEW.id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_create_user_settings_after_insert
    AFTER INSERT ON users
                    FOR EACH ROW EXECUTE FUNCTION create_default_user_settings();