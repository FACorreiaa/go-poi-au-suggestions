-- +migrate Up
-- Table for predefined interests/tags users can select
CREATE TABLE interests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    name CITEXT UNIQUE NOT NULL, -- 'History', 'Art', 'Foodie', 'Nightlife', 'Outdoors', 'Coffee', 'Museums', 'Shopping' etc.
    description TEXT,
    active bool DEFAULT true, -- Whether this interest is currently active
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ
);

-- Seed initial interests (optional, can be done via app logic too)
INSERT INTO
    interests (name, description)
VALUES (
        'History',
        'Historical sites, monuments, and stories'
    ),
    (
        'Art',
        'Art galleries, street art, museums with art collections'
    ),
    (
        'Foodie',
        'Restaurants, cafes, street food, culinary experiences'
    ),
    (
        'Nightlife',
        'Bars, clubs, live music venues'
    ),
    (
        'Outdoors',
        'Parks, hiking trails, nature spots'
    ),
    (
        'Coffee',
        'Specialty coffee shops and cafes'
    ),
    (
        'Museums',
        'General museums covering various topics'
    ),
    (
        'Shopping',
        'Boutiques, markets, shopping centers'
    ),
    (
        'Architecture',
        'Interesting buildings and structural design'
    ),
    (
        'Family Friendly',
        'Activities suitable for children and families'
    ),
    (
        'Off the Beaten Path',
        'Less touristy, unique spots'
    )
ON CONFLICT (name) DO NOTHING;

-- Join table for Many-to-Many relationship between users and interests
CREATE TABLE user_interests (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    interest_id UUID NOT NULL REFERENCES interests (id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, interest_id) -- Ensure a user can't have the same interest twice
);

-- Index for finding users by interest or interests by user
CREATE INDEX idx_user_interests_interest_id ON user_interests (interest_id);

CREATE INDEX idx_user_interests_user_id ON user_interests (user_id);

CREATE INDEX idx_interests_name ON interests (name);

CREATE TABLE user_custom_interests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name TEXT NOT NULL, -- Maybe CITEXT
    description TEXT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    -- preference_level INTEGER DEFAULT 1 CHECK (preference_level >= 0), -- Store level here?
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Constraint to prevent duplicate names *per user*
    CONSTRAINT fk_user_custom_interests_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT unique_user_custom_interest_name UNIQUE (user_id, name)
);

CREATE INDEX idx_user_custom_interests_user_name ON user_custom_interests (user_id, name);

CREATE TRIGGER trigger_set_user_custom_interests_updated_at
    BEFORE UPDATE ON user_custom_interests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Join table for Users saving Points of Interest (Many-to-Many)
CREATE TABLE saved_pois (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    poi_id UUID NOT NULL REFERENCES points_of_interest (id) ON DELETE CASCADE,
    saved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, poi_id) -- Ensure a user can't save the same POI twice
);

-- Index for finding POIs saved by a user, or users who saved a POI
CREATE INDEX idx_saved_pois_poi_id ON saved_pois (poi_id);