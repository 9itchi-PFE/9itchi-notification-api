package services

import (
	"context"
	"errors"

	"9itchi-notification-api/core"
	"9itchi-notification-api/pkg/account"
	"9itchi-notification-api/pkg/email"
	"9itchi-notification-api/repositories"
	"9itchi-notification-api/utils"
)

// Services holds service implementations and the peer clients they call.
type Services struct {
	Notification NotificationService
	// Consumer delivers the events of the other services to Notification.HandleEvent (no-op without RABBITMQ_URL).
	Consumer core.EventConsumer
	Clients  *Clients
}

// Clients holds one HTTP client per peer service (singletons, 50s timeout).
type Clients struct {
	Account *account.Client
	Email   *email.Client
}

func InitServices(config utils.AppConfig, repos *repositories.Repositories) *Services {
	realtime, tokens := core.NewRealtime(config.AblyAPIKey)
	accountClient := account.New(config.AccountAPIURL, config.InternalAPIKey)
	emailClient := email.New(config.EmailAPIURL, config.InternalAPIKey)
	channels := core.NewChannelFactory()
	channels.RegisterEmail(core.EmailConfig{
		APIKey:        config.MailgunAPIKey,
		Domain:        config.MailgunDomain,
		From:          config.MailgunFromEmail,
		APIBase:       config.MailgunAPIBase,
		RatePerSecond: config.MailgunRateLimit,
	}, func(ctx context.Context, userID string) (string, error) {
		u, err := accountClient.GetUser(ctx, userID)
		if err != nil {
			return "", err
		}
		return u.Email, nil
	})
	return &Services{
		Notification: NewNotificationService(repos.Notification, channels, realtime, tokens, WithAlerts(AlertDeps{
			Prefs:  repos.Preference,
			Mailer: channels.Mailer(),
			Users: func(ctx context.Context, userID string) (*Recipient, error) {
				u, err := accountClient.GetUser(ctx, userID)
				if err != nil {
					return nil, err
				}
				return &Recipient{Email: u.Email, Name: u.Name, Language: u.Language}, nil
			},
			EmailContext: func(ctx context.Context, emailID string) (*EmailContext, error) {
				c, err := emailClient.GetContext(ctx, emailID)
				if errors.Is(err, email.ErrNotFound) {
					return nil, nil
				}
				if err != nil {
					return nil, err
				}
				return &EmailContext{ApplicationID: c.ApplicationID, JobID: c.JobID, JobTitle: c.JobTitle, Company: c.Company}, nil
			},
			FrontendURL: config.FrontendURL,
		})),
		Consumer: core.NewEventConsumer(config.RabbitMQURL, EventsQueue, HandledEvents),
		Clients:  &Clients{Account: accountClient, Email: emailClient},
	}
}
