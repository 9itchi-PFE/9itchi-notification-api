package core

import "context"

// Channel is one delivery channel (in-app via Ably, email, push). Register in the factory.
type Channel interface {
	Name() string
	Send(ctx context.Context, userID, title string, data map[string]any) error
}

type ChannelFactory map[string]func() Channel
