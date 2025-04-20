-- +migrate Down
DROP TRIGGER IF EXISTS trigger_set_poi_updated_at ON points_of_interest;
DROP TRIGGER IF EXISTS trigger_set_users_updated_at ON users;

DROP INDEX IF EXISTS idx_poi_location;

DROP TABLE IF EXISTS points_of_interest;
DROP TABLE IF EXISTS users;

DROP FUNCTION IF EXISTS set_updated_at();