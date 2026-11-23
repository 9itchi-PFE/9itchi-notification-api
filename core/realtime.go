package core

import "context"

// Realtime pushes a payload to a user's live connection (Ably later). Failures never block the stored notification.
type Realtime interface {
	Publish(ctx context.Context, userID string, payload any) error
}

// NoopRealtime is the default until a real publisher is plugged in.
type NoopRealtime struct{}

func (NoopRealtime) Publish(_ context.Context, _ string, _ any) error { return nil }
