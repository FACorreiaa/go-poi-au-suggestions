-- +migrate Up

-- Add preference level to the user_interests join table
ALTER TABLE user_interests
ADD COLUMN preference_level INTEGER DEFAULT 1 CHECK (preference_level >= 0);
-- 0=Neutral/Nice-to-have, 1=Preferred, 2=Must-Have? Define levels

-- Optional: Could also use BOOLEAN 'is_required' DEFAULT FALSE

-- Create a global tags table (can be used for POIs, user avoids, etc.)
CREATE TABLE global_tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    name CITEXT UNIQUE NOT NULL, -- e.g., 'crowded', 'expensive', 'touristy', 'loud', 'requires_booking'
    description TEXT,
    tag_type TEXT NOT NULL DEFAULT 'general', -- e.g., 'vibe', 'cost', 'logistics', 'atmosphere'
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT unique_tag_name UNIQUE (name)
);

CREATE INDEX idx_global_tags_name ON global_tags (name);

CREATE INDEX idx_global_tags_active ON global_tags (active);
-- Seed some common avoid tags
INSERT INTO
    global_tags (name, tag_type, description)
VALUES (
        'crowded',
        'atmosphere',
        'Places known for being very busy'
    ),
    (
        'loud',
        'atmosphere',
        'Venues with high noise levels'
    ),
    (
        'expensive',
        'cost',
        'Significantly above average price'
    ),
    (
        'touristy',
        'atmosphere',
        'Primarily caters to large tourist groups'
    ),
    (
        'requires_booking',
        'logistics',
        'Booking/reservations typically essential'
    )
ON CONFLICT (name) DO NOTHING;

CREATE TABLE user_personal_tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    profile_id UUID NULL REFERENCES user_preference_profiles (id) ON DELETE CASCADE, -- Optional: Link to profile? Or just user?
    name TEXT NOT NULL,
    tag_type TEXT DEFAULT 'personal', -- Differentiate from global
    description TEXT,
    active bool DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    FOREIGN KEY (profile_id) REFERENCES user_preference_profiles (id) ON DELETE SET NULL,
    CONSTRAINT unique_user_tag_name UNIQUE (user_id, name)
);

CREATE INDEX idx_user_personal_tags_user_id ON user_personal_tags (user_id);

CREATE INDEX idx_user_personal_tags_profile_id ON user_personal_tags (profile_id);