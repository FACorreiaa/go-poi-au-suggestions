-- +migrate Up
CREATE TYPE search_pace_enum AS ENUM (
    'any',      -- No preference
    'relaxed',  -- Fewer, longer activities
    'moderate', -- Standard pace
    'fast'      -- Pack in many activities
    );

CREATE TYPE transport_preference_enum AS ENUM (
    'any',
    'walk',     -- Prefer easily walkable distances/areas
    'public',   -- Prefer locations easily accessible by public transport
    'car'       -- Assume user has a car, parking might be relevant
    );

CREATE TYPE day_preference_enum AS ENUM (
    'any',      -- No specific preference
    'day',      -- Primarily daytime activities (e.g., 8am - 6pm)
    'night'     -- Primarily evening/night activities (e.g., 6pm - 2am)
    );

-- Table for user-defined preference profiles
CREATE TABLE user_preference_profiles (
                                          id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                                          user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                          profile_name TEXT NOT NULL,                      -- e.g., "Weekend Getaway", "Default", "Business Lunch"
                                          is_default BOOLEAN NOT NULL DEFAULT FALSE,     -- Indicates if this is the user's default profile (only one should be true)
    -- Settings columns (mirroring the previous user_settings, but now per profile)
                                          search_radius_km NUMERIC(5, 1) DEFAULT 5.0 CHECK (search_radius_km > 0),
                                          preferred_time day_preference_enum DEFAULT 'any', -- Reuse enum from previous migration
                                          budget_level INTEGER DEFAULT 0 CHECK (budget_level >= 0 AND budget_level <= 4),
                                          preferred_pace search_pace_enum DEFAULT 'any',   -- Reuse enum from previous migration
                                          prefer_accessible_pois BOOLEAN DEFAULT FALSE,
                                          prefer_outdoor_seating BOOLEAN DEFAULT FALSE,
                                          prefer_dog_friendly    BOOLEAN DEFAULT FALSE,
    -- New advanced preferences
                                          preferred_vibes TEXT[] DEFAULT '{}',             -- Array of text tags: {'lively', 'quiet', 'romantic'}
                                          preferred_transport transport_preference_enum DEFAULT 'any',
                                          dietary_needs TEXT[] DEFAULT '{}',               -- Array of text tags: {'vegetarian', 'gluten_free'}

                                          created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                          updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Ensure a user can't have two profiles with the same name
                                          CONSTRAINT unique_user_profile_name UNIQUE (user_id, profile_name),
    -- Ensure only one profile can be the default per user (using a partial unique index)
                                          CONSTRAINT unique_default_profile UNIQUE (user_id) WHERE (is_default = TRUE)
);

-- Index for finding profiles by user, and the default profile quickly
-- Index for finding profiles by user
CREATE INDEX idx_user_preference_profiles_user_id ON user_preference_profiles (user_id);

-- *** ADD SEPARATE PARTIAL UNIQUE INDEX for is_default ***
-- Ensures only one profile can be the default per user
CREATE UNIQUE INDEX idx_user_preference_profiles_user_id_default
    ON user_preference_profiles (user_id)
    WHERE is_default = TRUE;

-- Trigger to update 'updated_at' timestamp
CREATE TRIGGER trigger_set_user_preference_profiles_updated_at
    BEFORE UPDATE ON user_preference_profiles
               FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Function to ensure only one default profile exists per user
-- This function will be called by triggers on INSERT and UPDATE
CREATE OR REPLACE FUNCTION ensure_single_default_profile()
    RETURNS TRIGGER AS $$
BEGIN
    -- If the inserted/updated row is being set as default
    IF NEW.is_default = TRUE THEN
        -- Set all other profiles for this user to NOT be default
        UPDATE user_preference_profiles
        SET is_default = FALSE
        WHERE user_id = NEW.user_id AND id != NEW.id; -- Exclude the current row being updated/inserted
END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to enforce single default on INSERT
CREATE TRIGGER trigger_enforce_single_default_insert
    AFTER INSERT ON user_preference_profiles
                    FOR EACH ROW EXECUTE FUNCTION ensure_single_default_profile();

-- Trigger to enforce single default on UPDATE
CREATE TRIGGER trigger_enforce_single_default_update
    AFTER UPDATE OF is_default ON user_preference_profiles -- Only run if is_default changes
    FOR EACH ROW
    WHEN (NEW.is_default = TRUE AND OLD.is_default = FALSE) -- Only when changing TO default
EXECUTE FUNCTION ensure_single_default_profile();


-- Function to create a default profile when a user is created
CREATE OR REPLACE FUNCTION create_initial_user_profile()
    RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO user_preference_profiles (user_id, profile_name, is_default)
    VALUES (NEW.id, 'Default', TRUE); -- Create a 'Default' profile marked as default
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to create default profile after user insert
CREATE TRIGGER trigger_create_user_profile_after_insert
    AFTER INSERT ON users
                    FOR EACH ROW EXECUTE FUNCTION create_initial_user_profile();

-- -- Drop the old user_settings table and its trigger function/trigger
-- DROP TRIGGER IF EXISTS trigger_create_user_settings_after_insert ON users;
-- DROP FUNCTION IF EXISTS create_default_user_settings();
-- DROP TABLE IF EXISTS user_settings;

-- Recreate user_interests to link PROFILE to INTEREST with a PREFERENCE LEVEL
CREATE TABLE user_profile_interests (
                                        profile_id UUID NOT NULL REFERENCES user_preference_profiles(id) ON DELETE CASCADE,
                                        interest_id UUID NOT NULL REFERENCES interests(id) ON DELETE CASCADE,
    -- Preference level for this interest WITHIN this specific profile
                                        preference_level INTEGER DEFAULT 1 NOT NULL CHECK (preference_level >= 0 AND preference_level <= 2), -- Example: 0=Nice, 1=Like, 2=Must-Have
                                        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Each profile can only have a specific interest listed once
                                        PRIMARY KEY (profile_id, interest_id)
);

CREATE INDEX idx_user_profile_interests_profile_id ON user_profile_interests (profile_id);
CREATE INDEX idx_user_profile_interests_interest_id ON user_profile_interests (interest_id);


