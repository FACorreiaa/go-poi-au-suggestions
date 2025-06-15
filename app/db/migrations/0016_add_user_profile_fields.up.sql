-- +migrate Up
-- Add missing fields to users table for enhanced user profiles

ALTER TABLE users 
ADD COLUMN location TEXT,
ADD COLUMN interests TEXT[], -- Array of user interests
ADD COLUMN badges TEXT[], -- Array of user badges
ADD COLUMN places_visited INTEGER DEFAULT 0,
ADD COLUMN reviews_written INTEGER DEFAULT 0,
ADD COLUMN lists_created INTEGER DEFAULT 0,
ADD COLUMN followers INTEGER DEFAULT 0,
ADD COLUMN following INTEGER DEFAULT 0;

-- Add indexes for performance
CREATE INDEX idx_users_location ON users (location);
CREATE INDEX idx_users_interests ON users USING gin(interests);
CREATE INDEX idx_users_badges ON users USING gin(badges);