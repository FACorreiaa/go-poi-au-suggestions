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