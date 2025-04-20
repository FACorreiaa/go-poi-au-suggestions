-- +migrate Up
CREATE OR REPLACE FUNCTION set_updated_at()
    RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Enum types for cleaner data constraints
CREATE TYPE subscription_plan_type AS ENUM (
    'free',
    'premium_monthly',
    'premium_annual'
);

CREATE TYPE subscription_status AS ENUM (
    'active',       -- Currently paid or free plan active
    'trialing',     -- In a trial period
    'past_due',     -- Payment failed
    'canceled',     -- Canceled by user, might still be active until end_date
    'expired'       -- Subscription period ended and not renewed
);

CREATE TYPE poi_source AS ENUM (
    'wanderwise_ai', -- Added by our system/AI
    'openstreetmap', -- Imported from OSM
    'user_submitted',-- Submitted by a user (maybe requires verification)
    'partner'        -- From a paying partner/featured listing
);