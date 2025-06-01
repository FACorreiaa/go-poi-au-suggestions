-- +migrate Up
-- Table for user-created collections of POIs
CREATE TABLE lists (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    image_url TEXT,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    is_itinerary BOOLEAN NOT NULL DEFAULT FALSE,
    city_id UUID REFERENCES cities (id) ON DELETE SET NULL,
    item_count INTEGER NOT NULL DEFAULT 0,
    view_count INTEGER NOT NULL DEFAULT 0,
    save_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Table for POIs in a list, with optional ordering for itineraries
CREATE TABLE list_items (
    list_id UUID NOT NULL REFERENCES lists (id) ON DELETE CASCADE,
    poi_id UUID NOT NULL REFERENCES points_of_interest (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    notes TEXT,
    day_number INTEGER, -- For itineraries (optional)
    time_slot TIMESTAMPTZ, -- For itineraries (optional)
    duration INTEGER, -- Duration in minutes (optional)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (list_id, poi_id)
);

-- Table for users saving other users' public lists
CREATE TABLE saved_lists (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    list_id UUID NOT NULL REFERENCES lists (id) ON DELETE CASCADE,
    saved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, list_id)
);

-- Indexes for efficient querying
CREATE INDEX idx_lists_user_id ON lists (user_id);

CREATE INDEX idx_lists_city_id ON lists (city_id);

CREATE INDEX idx_lists_is_public ON lists (is_public);

CREATE INDEX idx_list_items_poi_id ON list_items (poi_id);

CREATE INDEX idx_saved_lists_list_id ON saved_lists (list_id);

-- Trigger to update 'updated_at' timestamp for lists
CREATE TRIGGER trigger_set_lists_updated_at
BEFORE UPDATE ON lists
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Trigger to update 'updated_at' timestamp for list items
CREATE TRIGGER trigger_set_list_items_updated_at
BEFORE UPDATE ON list_items
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Trigger to update list item_count when items are added or removed
CREATE OR REPLACE FUNCTION update_list_item_count() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        -- Update list item count when an item is deleted
        UPDATE lists
        SET item_count = (
            SELECT COUNT(*)
            FROM list_items
            WHERE list_id = OLD.list_id
        )
        WHERE id = OLD.list_id;
        RETURN OLD;
    ELSE
        -- Update list item count when an item is inserted
        UPDATE lists
        SET item_count = (
            SELECT COUNT(*)
            FROM list_items
            WHERE list_id = NEW.list_id
        )
        WHERE id = NEW.list_id;
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_list_item_count_insert
AFTER INSERT ON list_items
FOR EACH ROW EXECUTE FUNCTION update_list_item_count();

CREATE TRIGGER trigger_update_list_item_count_delete
AFTER DELETE ON list_items
FOR EACH ROW EXECUTE FUNCTION update_list_item_count();

-- Trigger to update list save_count when a list is saved or unsaved
CREATE OR REPLACE FUNCTION update_list_save_count() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        -- Update list save count when a save is deleted
        UPDATE lists
        SET save_count = (
            SELECT COUNT(*)
            FROM saved_lists
            WHERE list_id = OLD.list_id
        )
        WHERE id = OLD.list_id;
        RETURN OLD;
    ELSE
        -- Update list save count when a save is inserted
        UPDATE lists
        SET save_count = (
            SELECT COUNT(*)
            FROM saved_lists
            WHERE list_id = NEW.list_id
        )
        WHERE id = NEW.list_id;
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_list_save_count_insert
AFTER INSERT ON saved_lists
FOR EACH ROW EXECUTE FUNCTION update_list_save_count();

CREATE TRIGGER trigger_update_list_save_count_delete
AFTER DELETE ON saved_lists
FOR EACH ROW EXECUTE FUNCTION update_list_save_count();