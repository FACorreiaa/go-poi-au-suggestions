-- +migrate Up
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    email CITEXT UNIQUE NOT NULL, -- Login identifier, case-insensitive
    username CITEXT UNIQUE, -- Optional display name, case-insensitive
    firstname CITEXT,
    lastname CITEXT,
    age int,
    city CITEXT,
    country CITEXT,
    about_you TEXT,
    theme TEXT,
    phone TEXT,
    language TEXT,
    role VARCHAR(50) NOT NULL DEFAULT 'user', -- e.g., 'admin', 'user', 'moderator'
    password_hash VARCHAR(255), -- Store hashed passwords only!
    display_name TEXT, -- Fallback display name if username is null
    profile_image_url TEXT, -- URL to user's avatar
    is_active BOOLEAN NOT NULL DEFAULT TRUE, -- For soft deletes or disabling accounts
    email_verified_at TIMESTAMPTZ, -- Timestamp when email was verified
    last_login_at TIMESTAMPTZ, -- Track last login time
    -- Preferences might be stored separately or as JSONB here
    -- preferences JSONB DEFAULT '{}'::jsonb, -- Option A: Simple, less queryable
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_providers (
    user_id UUID REFERENCES users (id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    provider_user_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (provider, provider_user_id)
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
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    user_id UUID UNIQUE NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Each user has one current subscription record
    plan subscription_plan_type NOT NULL DEFAULT 'free',
    status subscription_status NOT NULL DEFAULT 'active',
    start_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_date TIMESTAMPTZ, -- NULL if ongoing or free plan
    trial_end_date TIMESTAMPTZ, -- When a trial period expires
    external_provider TEXT, -- e.g., 'stripe', 'paypal'
    external_subscription_id TEXT UNIQUE, -- Subscription ID from the payment provider
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for faster lookup by user and status
CREATE INDEX idx_subscriptions_user_id ON subscriptions (user_id);

CREATE INDEX idx_subscriptions_status ON subscriptions (status);

CREATE INDEX idx_subscriptions_end_date ON subscriptions (end_date);
-- Useful for finding expired subs

-- Trigger to update 'updated_at' timestamp
CREATE TRIGGER trigger_set_subscriptions_updated_at
BEFORE UPDATE ON subscriptions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    user_id UUID NOT NULL,
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMPTZ,
    CONSTRAINT fk_user_refresh_token FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (), -- Internal row identifier
    session_id TEXT UNIQUE NOT NULL, -- The secure ID stored in cookie/header (used for lookup)
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Link to the user
    expires_at TIMESTAMPTZ NOT NULL, -- When the session automatically becomes invalid
    invalidated_at TIMESTAMPTZ, -- When the session was manually invalidated (e.g., logout), NULL if still valid until expiry
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- Tracks updates, e.g., extending session activity
    -- Optional: Add metadata like IP address or User-Agent if needed
    ip_address INET,
    user_agent TEXT
);

-- Indexes for efficient lookups and cleanup
CREATE INDEX idx_sessions_session_id ON sessions (session_id);
-- For finding session by its ID
CREATE INDEX idx_sessions_user_id ON sessions (user_id);
-- For finding sessions by user
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);
-- For cleaning up expired sessions