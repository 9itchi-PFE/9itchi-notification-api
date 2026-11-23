package services

import (
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
	return &Services{
		Notification: NewNotificationService(repos.Notification),
		Clients: &Clients{
			Account: account.New(config.AccountAPIURL, config.InternalAPIKey),
		},
	}
}
