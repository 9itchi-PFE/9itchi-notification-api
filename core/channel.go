package core

import "context"

// Channel is one delivery channel (in-app via Ably, email, push). Register in the factory.
type Channel interface {
	Name() string
	Send(ctx context.Context, userID, title string, data map[string]any) error
}

type ChannelFactory map[string]func() Channel

// ChannelInApp is the name of the default channel.
const ChannelInApp = "in_app"

// InAppChannel is the default channel: the stored DB row is the delivery, nothing more to do.
type InAppChannel struct{}

func (InAppChannel) Name() string { return ChannelInApp }

func (InAppChannel) Send(_ context.Context, _, _ string, _ map[string]any) error { return nil }

// NewChannelFactory returns the registry with the default channels.
func NewChannelFactory() ChannelFactory {
	return ChannelFactory{
		ChannelInApp: func() Channel { return InAppChannel{} },
	}
}
