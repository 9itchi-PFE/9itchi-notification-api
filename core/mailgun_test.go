package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mailgun/mailgun-go/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sentMail struct{ from, to, subject, text, html string }

type fakeSender struct {
	errs  []error // returned in order, nil afterwards
	calls []sentMail
}

func (f *fakeSender) send(_ context.Context, from, to, subject, text, htmlBody string) error {
	f.calls = append(f.calls, sentMail{from, to, subject, text, htmlBody})
	if len(f.errs) >= len(f.calls) {
		return f.errs[len(f.calls)-1]
	}
	return nil
}

func resolveTo(addr string) EmailResolver {
	return func(context.Context, string) (string, error) { return addr, nil }
}

func newTestChannel(s mailSender, resolve EmailResolver) *EmailChannel {
	c := newEmailChannel(s, "9itchi <no-reply@example.com>", 100, resolve)
	c.backoff = time.Millisecond
	return c
}

func TestRegisterEmailOnlyWithConfig(t *testing.T) {
	resolve := resolveTo("a@b.c")
	for _, cfg := range []EmailConfig{
		{},
		{APIKey: "key"},
		{Domain: "mg.example.com"},
	} {
		f := NewChannelFactory()
		f.RegisterEmail(cfg, resolve)
		assert.Len(t, f, 1)
		assert.NotContains(t, f, ChannelEmail)
	}

	f := NewChannelFactory()
	f.RegisterEmail(EmailConfig{APIKey: "key", Domain: "mg.example.com", APIBase: "https://api.eu.mailgun.net"}, resolve)
	require.Contains(t, f, ChannelEmail)
	assert.Equal(t, ChannelEmail, f[ChannelEmail]().Name())
	assert.Contains(t, f, ChannelInApp)
}

func TestEmailSendSuccessEscapesContent(t *testing.T) {
	s := &fakeSender{}
	err := newTestChannel(s, resolveTo("user@example.com")).Send(context.Background(), "uid", "Hi <b>there</b>", map[string]any{"body": "a & <script>x</script>\nline2"})
	require.NoError(t, err)
	require.Len(t, s.calls, 1)
	c := s.calls[0]
	assert.Equal(t, "user@example.com", c.to)
	assert.Equal(t, "9itchi <no-reply@example.com>", c.from)
	assert.Equal(t, "Hi <b>there</b>", c.subject)
	assert.Equal(t, "Hi <b>there</b>\n\na & <script>x</script>\nline2", c.text)
	assert.Equal(t, "<h3>Hi &lt;b&gt;there&lt;/b&gt;</h3><p>a &amp; &lt;script&gt;x&lt;/script&gt;<br>line2</p>", c.html)
}

func TestEmailSendUsesMessageFallback(t *testing.T) {
	s := &fakeSender{}
	require.NoError(t, newTestChannel(s, resolveTo("a@b.c")).Send(context.Background(), "t", "t", map[string]any{"message": "m"}))
	assert.Contains(t, s.calls[0].text, "m")
}

func TestEmailRetriesOn5xxThenSucceeds(t *testing.T) {
	s := &fakeSender{errs: []error{&mailgun.UnexpectedResponseError{Actual: 503}, &mailgun.UnexpectedResponseError{Actual: 500}}}
	require.NoError(t, newTestChannel(s, resolveTo("a@b.c")).Send(context.Background(), "u", "t", nil))
	assert.Len(t, s.calls, 3)
}

func TestEmailGivesUpAfterThreeAttempts(t *testing.T) {
	e := &mailgun.UnexpectedResponseError{Actual: 502}
	s := &fakeSender{errs: []error{e, e, e, e}}
	err := newTestChannel(s, resolveTo("a@b.c")).Send(context.Background(), "u", "t", nil)
	require.Error(t, err)
	assert.Len(t, s.calls, 3)
}

func TestEmailNoRetryOn4xx(t *testing.T) {
	s := &fakeSender{errs: []error{&mailgun.UnexpectedResponseError{Actual: 400}}}
	err := newTestChannel(s, resolveTo("a@b.c")).Send(context.Background(), "u", "t", nil)
	require.Error(t, err)
	assert.Len(t, s.calls, 1)
}

func TestEmailSubjectStripsLineBreaks(t *testing.T) {
	s := &fakeSender{}
	require.NoError(t, newTestChannel(s, resolveTo("a@b.c")).Send(context.Background(), "u", "Hi\r\nBcc: evil@x.y", nil))
	assert.Equal(t, "HiBcc: evil@x.y", s.calls[0].subject)
}

func TestEmailResolverFailureReturned(t *testing.T) {
	boom := errors.New("account down")
	s := &fakeSender{}
	err := newTestChannel(s, func(context.Context, string) (string, error) { return "", boom }).Send(context.Background(), "u", "t", nil)
	assert.ErrorIs(t, err, boom)
	assert.Empty(t, s.calls)
}

func TestEmailStopsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &fakeSender{}
	assert.Error(t, newTestChannel(s, resolveTo("a@b.c")).Send(ctx, "u", "t", nil))
	assert.Empty(t, s.calls)
}
