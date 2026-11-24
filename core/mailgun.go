package core

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net"
	"strings"
	"time"

	"github.com/mailgun/mailgun-go/v4"
	"golang.org/x/time/rate"
)

// ChannelEmail is the name of the Mailgun e-mail channel.
const ChannelEmail = "email"

const (
	emailMaxAttempts   = 3
	emailRetryDelay    = 500 * time.Millisecond
	emailCallTimeout   = 8 * time.Second
	emailTotalTimeout  = 20 * time.Second // caps recipient lookup + all attempts so a slow provider cannot hold the caller for long
	defaultEmailRate   = 10
	mailgunAPIVersion  = "/v3"
	defaultFromDisplay = "9itchi"
)

// EmailResolver returns the recipient address of a user (backed by the account peer client). Never store or log the result.
type EmailResolver func(ctx context.Context, userID string) (string, error)

// mailSender is the minimal Mailgun surface used here; tests replace it.
type mailSender interface {
	send(ctx context.Context, from, to, subject, text, htmlBody string) error
}

type mailgunSender struct {
	mg *mailgun.MailgunImpl
}

func (s mailgunSender) send(ctx context.Context, from, to, subject, text, htmlBody string) error {
	msg := mailgun.NewMessage(from, subject, text, to)
	msg.SetHTML(htmlBody)
	_, _, err := s.mg.Send(ctx, msg)
	return err
}

// EmailChannel e-mails the notification through Mailgun. Failures are returned to the caller, which never lets them block the stored notification.
type EmailChannel struct {
	sender  mailSender
	from    string
	resolve EmailResolver
	limiter *rate.Limiter
	backoff time.Duration
}

func (*EmailChannel) Name() string { return ChannelEmail }

// EmailConfig holds the plain Mailgun settings so core does not depend on the app config. APIKey is a secret: never log it.
type EmailConfig struct {
	APIKey        string
	Domain        string
	From          string // empty = "9itchi <no-reply@Domain>"
	APIBase       string // empty = US region
	RatePerSecond int    // < 1 = default
}

// RegisterEmail adds the email channel when APIKey and Domain are set; otherwise the factory is unchanged.
func (f ChannelFactory) RegisterEmail(cfg EmailConfig, resolve EmailResolver) {
	if cfg.APIKey == "" || cfg.Domain == "" {
		return
	}
	mg := mailgun.NewMailgun(cfg.Domain, cfg.APIKey)
	if base := strings.TrimRight(cfg.APIBase, "/"); base != "" {
		if !strings.HasSuffix(base, mailgunAPIVersion) {
			base += mailgunAPIVersion
		}
		mg.SetAPIBase(base)
	}
	from := cfg.From
	if from == "" {
		from = fmt.Sprintf("%s <no-reply@%s>", defaultFromDisplay, cfg.Domain)
	}
	ch := newEmailChannel(mailgunSender{mg: mg}, from, cfg.RatePerSecond, resolve)
	f[ChannelEmail] = func() Channel { return ch }
}

func newEmailChannel(sender mailSender, from string, ratePerSecond int, resolve EmailResolver) *EmailChannel {
	if ratePerSecond < 1 {
		ratePerSecond = defaultEmailRate
	}
	return &EmailChannel{
		sender:  sender,
		from:    from,
		resolve: resolve,
		limiter: rate.NewLimiter(rate.Limit(ratePerSecond), ratePerSecond),
		backoff: emailRetryDelay,
	}
}

func (c *EmailChannel) Send(ctx context.Context, userID, title string, data map[string]any) error {
	ctx, cancelAll := context.WithTimeout(ctx, emailTotalTimeout)
	defer cancelAll()
	title = headerSafe.Replace(title)
	to, err := c.resolve(ctx, userID)
	if err != nil {
		return fmt.Errorf("resolve recipient: %w", err)
	}
	if to == "" {
		return errors.New("resolve recipient: no email address")
	}
	text, htmlBody := renderEmail(title, data)

	var lastErr error
	for attempt := 1; attempt <= emailMaxAttempts; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("email rate limiter: %w", err)
		}
		callCtx, cancel := context.WithTimeout(ctx, emailCallTimeout)
		lastErr = c.sender.send(callCtx, c.from, to, title, text, htmlBody)
		cancel()
		if lastErr == nil {
			return nil
		}
		if !shouldRetry(lastErr) || attempt == emailMaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.backoff * time.Duration(attempt)):
		}
	}
	return fmt.Errorf("send email: %w", lastErr)
}

// headerSafe strips CR/LF from values used in the subject (header injection).
var headerSafe = strings.NewReplacer("\r", "", "\n", "")

// shouldRetry is true only for Mailgun 5xx responses and network errors.
func shouldRetry(err error) bool {
	var resp *mailgun.UnexpectedResponseError
	if errors.As(err, &resp) {
		return resp.Actual >= 500
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// renderEmail builds the plain-text and HTML bodies from the title and data["body"] (or data["message"]). Dynamic values are HTML-escaped.
func renderEmail(title string, data map[string]any) (text, htmlBody string) {
	body, _ := data["body"].(string)
	if body == "" {
		body, _ = data["message"].(string)
	}
	text = title
	htmlBody = "<h3>" + html.EscapeString(title) + "</h3>"
	if body != "" {
		text += "\n\n" + body
		htmlBody += "<p>" + strings.ReplaceAll(html.EscapeString(body), "\n", "<br>") + "</p>"
	}
	return text, htmlBody
}
