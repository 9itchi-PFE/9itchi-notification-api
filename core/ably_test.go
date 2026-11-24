package core

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
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
