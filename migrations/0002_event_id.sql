-- +goose Up
-- The id of the event a notification came from. An event delivered twice makes one notification.
ALTER TABLE notifications ADD COLUMN event_id text;
CREATE UNIQUE INDEX idx_notifications_event_id ON notifications (event_id) WHERE event_id IS NOT NULL;

-- +goose Down
DROP INDEX idx_notifications_event_id;
ALTER TABLE notifications DROP COLUMN event_id;
