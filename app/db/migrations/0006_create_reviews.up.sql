-- +migrate Up
-- Table for user reviews of POIs
CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    poi_id UUID NOT NULL REFERENCES points_of_interest (id) ON DELETE CASCADE,
    rating INTEGER NOT NULL CHECK (
        rating >= 1
        AND rating <= 5
    ),
    title TEXT,
    content TEXT NOT NULL,
    visit_date TIMESTAMPTZ,
    image_urls TEXT [],
    helpful INTEGER NOT NULL DEFAULT 0,
    unhelpful INTEGER NOT NULL DEFAULT 0,
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    is_published BOOLEAN NOT NULL DEFAULT TRUE,
    moderated_at TIMESTAMPTZ,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Table for tracking users marking reviews as helpful/unhelpful
CREATE TABLE review_helpfuls (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    review_id UUID NOT NULL REFERENCES reviews (id) ON DELETE CASCADE,
    is_helpful BOOLEAN NOT NULL, -- TRUE for helpful, FALSE for unhelpful
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, review_id) -- A user can only mark a review as helpful/unhelpful once
);

-- Table for replies to reviews
CREATE TABLE review_replies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    review_id UUID NOT NULL REFERENCES reviews (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    is_official BOOLEAN NOT NULL DEFAULT FALSE, -- TRUE for POI owner/staff replies
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX idx_reviews_poi_id ON reviews (poi_id);

CREATE INDEX idx_reviews_user_id ON reviews (user_id);

CREATE INDEX idx_reviews_rating ON reviews (rating);

CREATE INDEX idx_reviews_created_at ON reviews (created_at);

CREATE INDEX idx_review_replies_review_id ON review_replies (review_id);

-- Trigger to update 'updated_at' timestamp for reviews
CREATE TRIGGER trigger_set_reviews_updated_at
BEFORE UPDATE ON reviews
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Trigger to update 'updated_at' timestamp for review replies
CREATE TRIGGER trigger_set_review_replies_updated_at
BEFORE UPDATE ON review_replies
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Trigger to update POI average rating and rating count when a review is added, updated, or deleted
CREATE OR REPLACE FUNCTION update_poi_rating() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        -- Update POI rating when a review is deleted
        UPDATE points_of_interest
        SET average_rating = (
            SELECT COALESCE(AVG(rating), 0)
            FROM reviews
            WHERE poi_id = OLD.poi_id AND is_published = TRUE
        ),
        rating_count = (
            SELECT COUNT(*)
            FROM reviews
            WHERE poi_id = OLD.poi_id AND is_published = TRUE
        )
        WHERE id = OLD.poi_id;
        RETURN OLD;
    ELSE
        -- Update POI rating when a review is inserted or updated
        UPDATE points_of_interest
        SET average_rating = (
            SELECT COALESCE(AVG(rating), 0)
            FROM reviews
            WHERE poi_id = NEW.poi_id AND is_published = TRUE
        ),
        rating_count = (
            SELECT COUNT(*)
            FROM reviews
            WHERE poi_id = NEW.poi_id AND is_published = TRUE
        )
        WHERE id = NEW.poi_id;
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_poi_rating_insert
AFTER INSERT ON reviews
FOR EACH ROW EXECUTE FUNCTION update_poi_rating();

CREATE TRIGGER trigger_update_poi_rating_update
AFTER UPDATE OF rating, is_published ON reviews
FOR EACH ROW EXECUTE FUNCTION update_poi_rating();

CREATE TRIGGER trigger_update_poi_rating_delete
AFTER DELETE ON reviews
FOR EACH ROW EXECUTE FUNCTION update_poi_rating();