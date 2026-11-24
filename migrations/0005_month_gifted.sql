-- +goose Up
-- One more thing a student can be told: the team gave them a month.
ALTER TABLE notifications DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_type_check CHECK (type IN
  ('email_opened', 'scheduled_email_sent', 'email_failed', 'new_matching_jobs', 'saved_job_closing', 'cv_tip', 'month_gifted', 'admin_message', 'system'));

-- +goose Down
-- The older constraint cannot hold the rows of the new type.
DELETE FROM notifications WHERE type = 'month_gifted';
ALTER TABLE notifications DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_type_check CHECK (type IN
  ('email_opened', 'scheduled_email_sent', 'email_failed', 'new_matching_jobs', 'saved_job_closing', 'cv_tip', 'admin_message', 'system'));
