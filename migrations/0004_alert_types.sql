-- +goose Up
-- Two more things a student can be told: a saved offer closes soon, and the weekly advice on their CV.
ALTER TABLE notifications DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_type_check CHECK (type IN
  ('email_opened', 'scheduled_email_sent', 'email_failed', 'new_matching_jobs', 'saved_job_closing', 'cv_tip', 'admin_message', 'system'));

-- +goose Down
-- The older constraint cannot hold the rows of the two new types.
DELETE FROM notifications WHERE type IN ('saved_job_closing', 'cv_tip');
ALTER TABLE notifications DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_type_check CHECK (type IN
  ('email_opened', 'scheduled_email_sent', 'email_failed', 'new_matching_jobs', 'admin_message', 'system'));
