-- +migrate Up

-- Table to store user itineraries
CREATE TABLE itineraries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    user_id UUID REFERENCES users (id) ON DELETE CASCADE,
    city_id UUID REFERENCES cities (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Table to link POIs to itineraries with AI-generated descriptions
CREATE TABLE itinerary_pois (
    itinerary_id UUID REFERENCES itineraries (id) ON DELETE CASCADE,
    poi_id UUID REFERENCES points_of_interest (id) ON DELETE CASCADE,
    order_index INTEGER NOT NULL, -- To maintain the sequence of POIs in the itinerary
    ai_description TEXT, -- AI-generated description specific to this POI in this itinerary
    PRIMARY KEY (itinerary_id, poi_id)
);