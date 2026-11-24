package core

import "context"

// Realtime pushes a payload to a user's live connection (AblyRealtime when ABLY_API_KEY is set). Failures never block the stored notification.
type Realtime interface {
	Publish(ctx context.Context, userID string, payload any) error
}

// NoopRealtime is the default when no Ably key is configured.
type NoopRealtime struct{}

func (NoopRealtime) Publish(_ context.Context, _ string, _ any) error { return nil }
