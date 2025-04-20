-- +migrate Up
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email CITEXT UNIQUE NOT NULL,           -- Login identifier, case-insensitive
    username CITEXT UNIQUE,                  -- Optional display name, case-insensitive
    password_hash TEXT NOT NULL,            -- Store hashed passwords only!
    display_name TEXT,                      -- Fallback display name if username is null
    profile_image_url TEXT,                 -- URL to user's avatar
    is_active BOOLEAN NOT NULL DEFAULT TRUE,-- For soft deletes or disabling accounts
    email_verified_at TIMESTAMPTZ,          -- Timestamp when email was verified
    last_login_at TIMESTAMPTZ,              -- Track last login time
    -- Preferences might be stored separately or as JSONB here
    -- preferences JSONB DEFAULT '{}'::jsonb, -- Option A: Simple, less queryable
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add indexes for common lookups
CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_username ON users (username);
CREATE INDEX idx_users_created_at ON users (created_at);

-- Trigger to update 'updated_at' timestamp
CREATE TRIGGER trigger_set_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Table to manage user subscription status (Freemium model)
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- Each user has one current subscription record
    plan subscription_plan_type NOT NULL DEFAULT 'free',
    status subscription_status NOT NULL DEFAULT 'active',
    start_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_date TIMESTAMPTZ,                  -- NULL if ongoing or free plan
    trial_end_date TIMESTAMPTZ,            -- When a trial period expires
    external_provider TEXT,               -- e.g., 'stripe', 'paypal'
    external_subscription_id TEXT UNIQUE, -- Subscription ID from the payment provider
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for faster lookup by user and status
CREATE INDEX idx_subscriptions_user_id ON subscriptions (user_id);
CREATE INDEX idx_subscriptions_status ON subscriptions (status);
CREATE INDEX idx_subscriptions_end_date ON subscriptions (end_date); -- Useful for finding expired subs

-- Trigger to update 'updated_at' timestamp
CREATE TRIGGER trigger_set_subscriptions_updated_at
BEFORE UPDATE ON subscriptions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();