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


CREATE TABLE user_profile_avoid_tags (
                                         profile_id UUID NOT NULL REFERENCES user_preference_profiles(id) ON DELETE CASCADE,
                                         tag_id UUID NOT NULL REFERENCES global_tags(id) ON DELETE CASCADE,
                                         created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                         PRIMARY KEY (profile_id, tag_id)
);

CREATE INDEX idx_user_profile_avoid_tags_profile_id ON user_profile_avoid_tags (profile_id);
CREATE INDEX idx_user_profile_avoid_tags_tag_id ON user_profile_avoid_tags (tag_id);