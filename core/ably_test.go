package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRealtimeWithoutKeyIsNoop(t *testing.T) {
	rt, tokens := NewRealtime("")
	assert.Equal(t, NoopRealtime{}, rt)
	assert.Nil(t, tokens)
}

func TestNewRealtimeWithKeyIsAbly(t *testing.T) {
	rt, tokens := NewRealtime("appid.keyid:secret")
	assert.IsType(t, &AblyRealtime{}, rt)
	assert.NotNil(t, tokens)
}

func TestRedactingAblyLoggerHidesKey(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	redactingAblyLogger{apiKey: "appid.keyid:secret"}.Printf(ably.LogError, "dial wss://x/?key=%s&v=1 failed for %s", "appid.keyid:secret", "appid.keyid:secret")

	assert.NotContains(t, buf.String(), "secret")
	assert.Contains(t, buf.String(), "key=REDACTED")
}

func TestUserChannel(t *testing.T) {
	assert.Equal(t, "user:abc", UserChannel("abc"))
}

// An Ably app that requires identified clients refuses a publish made with the key alone: the publisher must say
// who it is, and the tokens it signs must still be for the user asked for, never for the service.
func TestPublishIsIdentifiedAndTokensAreNot(t *testing.T) {
	var clientID, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID, path = r.Header.Get("X-Ably-ClientId"), r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)

	rt, tokens := newRealtime("appid.keyid:secret", ably.WithEndpoint(u.Hostname()), ably.WithPort(port), ably.WithTLS(false),
		ably.WithInsecureAllowBasicAuthWithoutTLS(), ably.WithUseBinaryProtocol(false))
	require.IsType(t, &AblyRealtime{}, rt)

	require.NoError(t, rt.Publish(context.Background(), "abc", map[string]any{"title": "hello"}))
	sent, err := base64.StdEncoding.DecodeString(clientID)
	require.NoError(t, err)
	assert.Equal(t, PublisherClientID, string(sent))
	assert.Contains(t, path, "abc")

	forUser, err := tokens.CreateTokenRequest(&ably.TokenParams{ClientID: "user-1"})
	require.NoError(t, err)
	assert.Equal(t, "user-1", forUser.ClientID)
	anonymous, err := tokens.CreateTokenRequest(&ably.TokenParams{})
	require.NoError(t, err)
	assert.Empty(t, anonymous.ClientID, "a token never inherits the service's identity")
}
