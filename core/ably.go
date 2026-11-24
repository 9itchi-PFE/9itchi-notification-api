package core

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"9itchi-notification-api/utils"

	"github.com/ably/ably-go/ably"
)

const ablyPublishTimeout = 10 * time.Second

// UserChannel is the Ably channel a user subscribes to for live notifications.
func UserChannel(userID string) string { return "user:" + userID }

// TokenRequestCreator is satisfied by *ably.Auth; tests replace it so no network call is made.
type TokenRequestCreator interface {
	CreateTokenRequest(params *ably.TokenParams, opts ...ably.AuthOption) (*ably.TokenRequest, error)
}

// AblyRealtime publishes notifications to the user's Ably channel over the REST API.
type AblyRealtime struct {
	rest *ably.REST
}

func (a *AblyRealtime) Publish(ctx context.Context, userID string, payload any) error {
	ctx, cancel := context.WithTimeout(ctx, ablyPublishTimeout)
	defer cancel()
	return a.rest.Channels.Get(UserChannel(userID)).Publish(ctx, "notification", payload)
}

// PublisherClientID is the identity this service publishes under. An Ably app can require identified clients on
// its channels; a publish made with the key alone is then refused (401, "requires an identified client").
const PublisherClientID = "9itchi-notification-api"

// NewRealtime is the Realtime factory: Ably when apiKey is set, else the no-op. The creator is nil without Ably.
func NewRealtime(apiKey string) (Realtime, TokenRequestCreator) {
	return newRealtime(apiKey)
}

// newRealtime builds two REST clients on the same key. The publisher is identified. Token requests are signed by a
// client without an identity, so a token is only ever issued for the client id asked for, never for the service's.
func newRealtime(apiKey string, extra ...ably.ClientOption) (Realtime, TokenRequestCreator) {
	if apiKey == "" {
		return NoopRealtime{}, nil
	}
	opts := append([]ably.ClientOption{ably.WithKey(apiKey), ably.WithLogHandler(redactingAblyLogger{apiKey: apiKey})}, extra...)
	signer, err := ably.NewREST(opts...)
	if err != nil {
		utils.Logger.Error("ably disabled: invalid ABLY_API_KEY")
		return NoopRealtime{}, nil
	}
	publisher, err := ably.NewREST(append(opts[:len(opts):len(opts)], ably.WithClientID(PublisherClientID))...)
	if err != nil {
		utils.Logger.Error("ably disabled: invalid ABLY_API_KEY")
		return NoopRealtime{}, nil
	}
	return &AblyRealtime{rest: publisher}, signer.Auth
}

var ablyKeyParam = regexp.MustCompile(`key=[^&\s"]+`)

// redactingAblyLogger strips the API key (it can appear in URLs on connection errors) before logging.
type redactingAblyLogger struct {
	apiKey string
}

func (l redactingAblyLogger) Printf(level ably.LogLevel, format string, v ...interface{}) {
	msg := ablyKeyParam.ReplaceAllString(fmt.Sprintf(format, v...), "key=REDACTED")
	if l.apiKey != "" {
		msg = strings.ReplaceAll(msg, l.apiKey, "REDACTED")
	}
	log.Printf("[%s] %s", level, msg)
}
