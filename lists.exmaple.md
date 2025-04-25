Okay, let's implement the database schema changes for those advanced preference features and then delve deeper into the workflow of using global vs. per-search preferences.

**I. Database Schema Implementation for Advanced Preferences**

We'll create new migration files to add these features.

**Migration 1: Preference Profiles, Vibes, Transport, Dietary (`migrations/<next_version>_add_preference_profiles.up.sql`)**

```sql
-- +migrate Up

-- ENUM type for transport preferences
CREATE TYPE transport_preference_enum AS ENUM (
    'any',
    'walk',     -- Prefer easily walkable distances/areas
    'public',   -- Prefer locations easily accessible by public transport
    'car'       -- Assume user has a car, parking might be relevant
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
CREATE INDEX idx_user_preference_profiles_user_id ON user_preference_profiles (user_id);
CREATE INDEX idx_user_preference_profiles_user_id_default ON user_preference_profiles (user_id, is_default) WHERE is_default = TRUE;

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

-- Drop the old user_settings table and its trigger function/trigger
DROP TRIGGER IF EXISTS trigger_create_user_settings_after_insert ON users;
DROP FUNCTION IF EXISTS create_default_user_settings();
DROP TABLE IF EXISTS user_settings;

```

**Migration 2: Interest Levels & Avoid Tags (`migrations/<next_version>_enhance_interests.up.sql`)**

```sql
-- +migrate Up

-- Add preference level to the user_interests join table
ALTER TABLE user_interests
ADD COLUMN preference_level INTEGER DEFAULT 1 CHECK (preference_level >= 0); -- 0=Neutral/Nice-to-have, 1=Preferred, 2=Must-Have? Define levels

-- Optional: Could also use BOOLEAN 'is_required' DEFAULT FALSE

-- Create a global tags table (can be used for POIs, user avoids, etc.)
CREATE TABLE global_tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name CITEXT UNIQUE NOT NULL,             -- e.g., 'crowded', 'expensive', 'touristy', 'loud', 'requires_booking'
    description TEXT,
    tag_type TEXT NOT NULL DEFAULT 'general', -- e.g., 'vibe', 'cost', 'logistics', 'atmosphere'
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Seed some common avoid tags
INSERT INTO global_tags (name, tag_type, description) VALUES
('crowded', 'atmosphere', 'Places known for being very busy'),
('loud', 'atmosphere', 'Venues with high noise levels'),
('expensive', 'cost', 'Significantly above average price'),
('touristy', 'atmosphere', 'Primarily caters to large tourist groups'),
('requires_booking', 'logistics', 'Booking/reservations typically essential')
ON CONFLICT (name) DO NOTHING;


-- Join table for Many-to-Many relationship between users and tags they want to AVOID
CREATE TABLE user_avoid_tags (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES global_tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, tag_id) -- Ensure user can't avoid same tag twice
);

-- Index for finding users by avoided tag or tags avoided by user
CREATE INDEX idx_user_avoid_tags_tag_id ON user_avoid_tags (tag_id);

-- Also add an index on user_interests user_id now that it's modified
CREATE INDEX idx_user_interests_user_id ON user_interests (user_id);

```

**Explanation of Schema Changes:**

*   **`user_preference_profiles`:** Replaces `user_settings`. Each user can have multiple named profiles (e.g., 'Default', 'Weekend Trip'). The `is_default` flag (with a unique partial index and triggers) ensures only one can be the active default. It includes the new preference columns (`preferred_vibes`, `preferred_transport`, `dietary_needs` as `TEXT[]` arrays).
*   **`user_interests`:** Added `preference_level` (INTEGER). You'll define the meaning (e.g., 0=Low, 1=Medium, 2=High/Must-Have). Alternatively, a `BOOLEAN` `is_required` could work.
*   **`global_tags`:** A central place for various descriptive tags beyond the main "interests". Useful for vibes, costs, logistics, and the "avoid" feature.
*   **`user_avoid_tags`:** Many-to-many join table linking users to tags from `global_tags` they want to avoid.

**II. Deeper Dive: Global vs. Per-Search Preference Workflow**

Let's elaborate on how the backend uses these preferences based on user actions.

**Scenario A: User opens the app / lands on a search page (NO specific search query yet)**

1.  **Frontend:** Loads the default view. To personalize this initial view (e.g., showing "Suggestions near you based on your preferences"), it needs the user's *default* profile.
2.  **Backend Request:** Frontend calls an endpoint like `GET /api/v1/users/me/default-profile` (or similar).
3.  **Backend Service (`UserService`):**
    *   Gets the authenticated `userID` from the context.
    *   Calls `UserRepo.GetDefaultPreferenceProfile(ctx, userID)` (this repo method would query `user_preference_profiles WHERE user_id = $1 AND is_default = TRUE`).
    *   Calls `UserRepo.GetUserPreferences(ctx, userID)` to get the associated *interest* IDs/names (and potentially their levels).
    *   Calls `UserRepo.GetUserAvoidTags(ctx, userID)` (a new repo method needed for the avoid list).
    *   Combines this profile data (radius, time, budget, vibes, dietary, interests, avoid tags) into a response DTO.
4.  **Frontend:** Receives the default profile and uses it to:
    *   Pre-populate filter UI elements.
    *   Potentially make an initial recommendation request using these defaults (`GET /api/v1/recommendations?use_default_profile=true&lat=...&lon=...`).

**Scenario B: User performs a specific search WITHOUT modifying filters**

1.  **Frontend:** User enters "Berlin" and hits search. The filter UI is showing their *default* preferences (pre-populated as in Scenario A). Since they haven't changed anything, the frontend constructs the request using these defaults.
2.  **Backend Request:** `GET /api/v1/recommendations?query=Berlin&interests=History,Art&max_distance=5.0&preferred_time=any&...` (Sends the *values* from the default profile as explicit parameters).
3.  **Backend Service (`RecommendationService`):**
    *   Receives the explicit parameters from the request (`query=Berlin`, `interests=History,Art`, `max_distance=5.0`, etc.).
    *   Uses *these request parameters* directly to query the database (`POIRepo.FindPOIs` or similar, filtering by location, interests, distance, etc.).
    *   Constructs the prompt for Gemini using the original query ("Berlin") and the filters applied ("looking for History and Art within 5km"). It might *also* add a note about the user's *other* global preferences (e.g., "User generally prefers 'quiet' vibes and 'vegetarian' options") as extra context for the LLM's textual response, even if those weren't primary filters for the POI query itself.
4.  **Frontend:** Displays results based on the parameters sent.

**Scenario C: User performs a specific search AND modifies filters**

1.  **Frontend:** User enters "Berlin". Filters pre-populate with defaults ('History', 'Art', 5km). User *unchecks* 'History', *adds* 'Coffee', and changes the distance slider to `2.0`km. They hit search.
2.  **Backend Request:** Frontend constructs the request based on the *current state* of the UI filters: `GET /api/v1/recommendations?query=Berlin&interests=Art,Coffee&max_distance=2.0&...` (Note: 'History' is *not* sent, 'Coffee' *is* sent, distance is 2.0).
3.  **Backend Service (`RecommendationService`):**
    *   Receives the *modified* parameters (`interests=Art,Coffee`, `max_distance=2.0`, etc.).
    *   Uses *these request parameters* to query the database (finds POIs matching 'Art' OR 'Coffee' within 2km).
    *   Constructs the prompt for Gemini using the original query ("Berlin") and the *active* filters ("looking for Art and Coffee within 2km"). Again, it could optionally add context about the user's broader default profile ("User generally also likes History and prefers quiet vibes...") to help the LLM generate a nuanced response.
4.  **Frontend:** Displays results based on the *overridden* parameters sent.

**Key Workflow Points:**

*   **Frontend Drives Per-Search Filters:** The frontend is responsible for managing the state of the search filters for the *current* search, starting with defaults and reflecting user modifications. It sends the *final set* of filters with the request.
*   **Backend Trusts Request Parameters:** The backend's primary filtering logic for fetching POIs relies on the parameters *received in the request*, not necessarily the user's globally saved defaults (though defaults are used if no overrides are sent).
*   **Global Preferences as AI Context:** Even when filters are overridden for POI selection, the backend can still leverage the user's full default profile (all interests, vibes, avoid tags, etc.) as valuable context when constructing the final prompt for the Gemini LLM to generate a more personalized *textual* summary or reasoning.

This hybrid approach provides flexibility for the user while allowing the backend and AI to leverage both immediate search intent and broader user preferences. You'll need corresponding repository methods (`GetUserDefaultPreferenceProfile`, `UpdatePreferenceProfile`, `Add/RemoveUserAvoidTag`, etc.) and service logic to manage these new entities.