-- +migrate Up
-- Enhanced user preferences for multi-domain filtering
-- Based on SEARCH_FILTERS_SUGGESTIONS.md recommendations

-- Create domain-specific preference tables following Option 1 approach

-- Domain-specific preference tables
CREATE TABLE user_accommodation_preferences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_preference_profile_id UUID REFERENCES user_preference_profiles(id) ON DELETE CASCADE,
    accommodation_filters JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_dining_preferences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_preference_profile_id UUID REFERENCES user_preference_profiles(id) ON DELETE CASCADE,
    dining_filters JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_activity_preferences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_preference_profile_id UUID REFERENCES user_preference_profiles(id) ON DELETE CASCADE,
    activity_filters JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_itinerary_preferences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_preference_profile_id UUID REFERENCES user_preference_profiles(id) ON DELETE CASCADE,
    itinerary_filters JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for better performance
CREATE INDEX idx_user_accommodation_preferences_profile_id ON user_accommodation_preferences(user_preference_profile_id);
CREATE INDEX idx_user_dining_preferences_profile_id ON user_dining_preferences(user_preference_profile_id);
CREATE INDEX idx_user_activity_preferences_profile_id ON user_activity_preferences(user_preference_profile_id);
CREATE INDEX idx_user_itinerary_preferences_profile_id ON user_itinerary_preferences(user_preference_profile_id);

-- Create JSONB indexes for filter queries
CREATE INDEX idx_user_accommodation_filters_gin ON user_accommodation_preferences USING GIN (accommodation_filters);
CREATE INDEX idx_user_dining_filters_gin ON user_dining_preferences USING GIN (dining_filters);
CREATE INDEX idx_user_activity_filters_gin ON user_activity_preferences USING GIN (activity_filters);
CREATE INDEX idx_user_itinerary_filters_gin ON user_itinerary_preferences USING GIN (itinerary_filters);

-- Add updated_at triggers
CREATE TRIGGER trigger_set_user_accommodation_preferences_updated_at
    BEFORE UPDATE ON user_accommodation_preferences
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_set_user_dining_preferences_updated_at
    BEFORE UPDATE ON user_dining_preferences
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_set_user_activity_preferences_updated_at
    BEFORE UPDATE ON user_activity_preferences
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_set_user_itinerary_preferences_updated_at
    BEFORE UPDATE ON user_itinerary_preferences
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Create function to initialize default preferences for existing profiles
CREATE OR REPLACE FUNCTION create_default_domain_preferences()
RETURNS TRIGGER AS $$
BEGIN
    -- Create default accommodation preferences
    INSERT INTO user_accommodation_preferences (user_preference_profile_id, accommodation_filters)
    VALUES (NEW.id, '{
        "accommodation_type": ["hotel", "apartment"],
        "star_rating": {"min": 3, "max": 5},
        "price_range_per_night": {"min": 0, "max": 300},
        "amenities": ["wifi"],
        "room_type": ["double"],
        "chain_preference": "any",
        "cancellation_policy": ["free_cancellation"],
        "booking_flexibility": "any"
    }');

    -- Create default dining preferences
    INSERT INTO user_dining_preferences (user_preference_profile_id, dining_filters)
    VALUES (NEW.id, '{
        "cuisine_types": ["local_specialty"],
        "meal_types": ["lunch", "dinner"],
        "service_style": ["casual"],
        "price_range_per_person": {"min": 0, "max": 100},
        "dietary_needs": [],
        "allergen_free": [],
        "michelin_rated": false,
        "local_recommendations": true,
        "chain_vs_local": "local_preferred",
        "organic_preference": false,
        "outdoor_seating_preferred": false
    }');

    -- Create default activity preferences
    INSERT INTO user_activity_preferences (user_preference_profile_id, activity_filters)
    VALUES (NEW.id, '{
        "activity_categories": ["museums", "history"],
        "physical_activity_level": "moderate",
        "indoor_outdoor_preference": "mixed",
        "cultural_immersion_level": "moderate",
        "must_see_vs_hidden_gems": "mixed",
        "educational_preference": true,
        "photography_opportunities": true,
        "season_specific_activities": ["year_round"],
        "avoid_crowds": false,
        "local_events_interest": ["cultural_events"]
    }');

    -- Create default itinerary preferences
    INSERT INTO user_itinerary_preferences (user_preference_profile_id, itinerary_filters)
    VALUES (NEW.id, '{
        "planning_style": "flexible",
        "preferred_pace": "moderate",
        "time_flexibility": "loose_schedule",
        "morning_vs_evening": "flexible",
        "weekend_vs_weekday": "any",
        "preferred_seasons": ["spring", "summer", "fall"],
        "avoid_peak_season": false,
        "adventure_vs_relaxation": "balanced",
        "spontaneous_vs_planned": "semi_planned"
    }');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Add trigger to create default domain preferences when a profile is created
CREATE TRIGGER trigger_create_default_domain_preferences_after_insert
    AFTER INSERT ON user_preference_profiles
    FOR EACH ROW EXECUTE FUNCTION create_default_domain_preferences();

-- Initialize domain preferences for existing profiles
INSERT INTO user_accommodation_preferences (user_preference_profile_id, accommodation_filters)
SELECT id, '{
    "accommodation_type": ["hotel", "apartment"],
    "star_rating": {"min": 3, "max": 5},
    "price_range_per_night": {"min": 0, "max": 300},
    "amenities": ["wifi"],
    "room_type": ["double"],
    "chain_preference": "any",
    "cancellation_policy": ["free_cancellation"],
    "booking_flexibility": "any"
}'
FROM user_preference_profiles 
WHERE id NOT IN (SELECT user_preference_profile_id FROM user_accommodation_preferences);

INSERT INTO user_dining_preferences (user_preference_profile_id, dining_filters)
SELECT id, '{
    "cuisine_types": ["local_specialty"],
    "meal_types": ["lunch", "dinner"],
    "service_style": ["casual"],
    "price_range_per_person": {"min": 0, "max": 100},
    "dietary_needs": [],
    "allergen_free": [],
    "michelin_rated": false,
    "local_recommendations": true,
    "chain_vs_local": "local_preferred",
    "organic_preference": false,
    "outdoor_seating_preferred": false
}'
FROM user_preference_profiles 
WHERE id NOT IN (SELECT user_preference_profile_id FROM user_dining_preferences);

INSERT INTO user_activity_preferences (user_preference_profile_id, activity_filters)
SELECT id, '{
    "activity_categories": ["museums", "history"],
    "physical_activity_level": "moderate",
    "indoor_outdoor_preference": "mixed",
    "cultural_immersion_level": "moderate",
    "must_see_vs_hidden_gems": "mixed",
    "educational_preference": true,
    "photography_opportunities": true,
    "season_specific_activities": ["year_round"],
    "avoid_crowds": false,
    "local_events_interest": ["cultural_events"]
}'
FROM user_preference_profiles 
WHERE id NOT IN (SELECT user_preference_profile_id FROM user_activity_preferences);

INSERT INTO user_itinerary_preferences (user_preference_profile_id, itinerary_filters)
SELECT id, '{
    "planning_style": "flexible",
    "preferred_pace": "moderate",
    "time_flexibility": "loose_schedule",
    "morning_vs_evening": "flexible",
    "weekend_vs_weekday": "any",
    "preferred_seasons": ["spring", "summer", "fall"],
    "avoid_peak_season": false,
    "adventure_vs_relaxation": "balanced",
    "spontaneous_vs_planned": "semi_planned"
}'
FROM user_preference_profiles 
WHERE id NOT IN (SELECT user_preference_profile_id FROM user_itinerary_preferences);

-- Add comments to explain the JSONB structure
COMMENT ON COLUMN user_accommodation_preferences.accommodation_filters IS 'JSONB containing accommodation-specific filters like accommodation_type, star_rating, price_range_per_night, amenities, etc.';
COMMENT ON COLUMN user_dining_preferences.dining_filters IS 'JSONB containing dining-specific filters like cuisine_types, meal_types, service_style, dietary_needs, etc.';
COMMENT ON COLUMN user_activity_preferences.activity_filters IS 'JSONB containing activity-specific filters like activity_categories, physical_activity_level, cultural_immersion_level, etc.';
COMMENT ON COLUMN user_itinerary_preferences.itinerary_filters IS 'JSONB containing itinerary-specific filters like planning_style, time_flexibility, preferred_seasons, etc.';