-- +migrate Up
CREATE TABLE points_of_interest (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    description TEXT,
    location GEOMETRY(Point, 4326), -- Uses PostGIS geometry type (SRID 4326 for WGS84)
    embedding VECTOR(768),          -- Uses pgvector type (adjust dimension)
    -- other poi fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trigger_set_poi_updated_at
BEFORE UPDATE ON points_of_interest
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_poi_location ON points_of_interest USING GIST (location);
