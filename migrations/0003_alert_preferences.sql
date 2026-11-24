-- +goose Up
-- What a user wants to be told, and where. A kind without a row keeps its default (core.DefaultAlert).
CREATE TABLE alert_preferences (
  user_id uuid NOT NULL,
  kind text NOT NULL CHECK (kind IN ('applications', 'matches', 'closing', 'tips')),
  email boolean NOT NULL,
  in_app boolean NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, kind)
);

-- +goose Down
DROP TABLE alert_preferences;
