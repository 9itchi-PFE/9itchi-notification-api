package core

import (
	"context"

	"9itchi-notification-api/utils"

	"go.uber.org/zap"
)

// Event names exchanged over RabbitMQ (exchange "9itchi.events"). Never rename a published name.
const (
	EventUserRegistered      = "user.registered"
	EventEmailSent           = "email.sent"
	EventEmailFailed         = "email.failed"
	EventEmailOpened         = "email.opened"
	EventJobsMatched         = "jobs.matched"
	EventSubscriptionCreated = "subscription.created"
	EventPaymentCompleted    = "payment.completed"
	// EventSubscriptionRenewalDue: a renewing subscription ends soon (stage "due") or ended without a renewal ("lapsed").
	EventSubscriptionRenewalDue = "subscription.renewal_due"
	// EventJobClosingSoon: an offer a student saved closes within a few days (job-api, once per saved offer).
	EventJobClosingSoon = "job.closing_soon"
	// EventCVTips: the check of a student's CV still finds points to correct (resume-api, once a week at most).
	EventCVTips = "cv.tips"
	// EventSubscriptionGifted: the team gave a student free days of a plan (payments-api); ends_at is the new end.
	EventSubscriptionGifted = "subscription.gifted"
)

// Publisher sends domain events. Swap LogPublisher for a RabbitMQ implementation without touching services.
type Publisher interface {
	Publish(ctx context.Context, event string, payload any) error
}

// LogPublisher logs the event name only (never the payload). Default until RabbitMQ is wired.
type LogPublisher struct{}

func (LogPublisher) Publish(_ context.Context, event string, _ any) error {
	utils.Logger.Info("event", zap.String("name", event))
	return nil
}
