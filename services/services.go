package services

import (
	"context"

	"9itchi-notification-api/core"
	"9itchi-notification-api/pkg/account"
	"9itchi-notification-api/repositories"
	"9itchi-notification-api/utils"
)

// Services holds service implementations and the peer clients they call.
type Services struct {
	Notification NotificationService
	Clients      *Clients
}

// Clients holds one HTTP client per peer service (singletons, 50s timeout).
type Clients struct {
	Account *account.Client
}

func InitServices(config utils.AppConfig, repos *repositories.Repositories) *Services {
	realtime, tokens := core.NewRealtime(config.AblyAPIKey)
	accountClient := account.New(config.AccountAPIURL, config.InternalAPIKey)
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
		Notification: NewNotificationService(repos.Notification, channels, realtime, tokens),
		Clients:      &Clients{Account: accountClient},
	}
}
