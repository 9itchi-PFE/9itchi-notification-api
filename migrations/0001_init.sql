-- +goose Up
CREATE TABLE notifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL,
  type text NOT NULL CHECK (type IN ('email_opened', 'scheduled_email_sent', 'email_failed', 'new_matching_jobs', 'admin_message', 'system')),
  title text NOT NULL,
  data jsonb NOT NULL DEFAULT '{}',
  read boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user_read_created ON notifications (user_id, read, created_at DESC);

-- +goose Down
DROP TABLE notifications;
