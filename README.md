# 9itchi-notification-api

In-app notifications delivered over Ably, optional email/push channels. Consumes events.

Port `8087`, health `/health`. Notifications are stored in PostgreSQL. Live push and the token endpoint use Ably when `ABLY_API_KEY` is set; without it `core.Realtime` is a no-op and `GET /notifications/ably-token` returns `503` `{"error":"realtime unavailable"}`. A publish failure never blocks the stored notification. The service publishes under its own Ably client id (`core.PublisherClientID`), because the Ably app requires identified clients; the tokens it signs are for the caller only.

## Endpoints

| Method | Path | Access | Handler |
|---|---|---|---|
| GET | `/notifications` | user | ListNotifications (`page`, `page_size`, `unread=true`; returns `unread_count`) |
| PUT | `/notifications/:id/read` | user | MarkRead (404 if not yours) |
| PUT | `/notifications/read-all` | user | MarkAllRead |
| GET | `/notifications/ably-token` | user | AblyToken (Ably TokenRequest, 30 min TTL, `clientId` = caller, subscribe only on `user:{id}`; 503 without `ABLY_API_KEY`) |
| GET | `/notifications/preferences` | user | GetAlertPreferences (the four kinds of alert with `email` and `in_app`) |
| PUT | `/notifications/preferences` | user | UpdateAlertPreferences (`items`: the kinds to change; the others keep their choice) |
| POST | `/internal/notifications` | internal | SendNotification (`user_id`, `type`, `title`, `data`, optional `channels`) |

Types: `email_opened`, `scheduled_email_sent`, `email_failed`, `new_matching_jobs`, `admin_message`, `system`.

## Events (RabbitMQ)

Optional. With `RABBITMQ_URL` set, the service reads the durable queue `notification-api.events`, bound on the topic exchange `9itchi.events` (routing key = event name). Without the variable nothing is consumed.

| Event | What happens |
|---|---|
| `email.opened` | notification `email_opened` + email, by the user's choice for `applications` |
| `email.sent` with `scheduled: true` | notification `scheduled_email_sent` + email, by the user's choice for `applications` |
| `email.failed` with `scheduled: true` | notification `email_failed` + email, always (an application that could not leave is not something to switch off) |
| `jobs.matched` | notification `new_matching_jobs`, by the user's choice for `matches` |
| `payment.completed` | the receipt by email, always (nothing for a payment of 0) |
| `user.registered` | the welcome email, always |
| `subscription.renewal_due` | the renewal reminder by email, always: `stage` `due` (the period ends soon) or `lapsed` (it ended without a renewal) |

- `email.sent` and `email.failed` say nothing when the student clicked the send: its result is on the screen.
- For the three application events the offer is read from email-api (`GET /internal/emails/:id/context`), so the notification data carries `company`, `job_title`, `application_id` and `job_id`, and the email says « Acme a ouvert ta candidature ». Without it (email-api down, or an email sent for no application) both are told without the name.
- An event is handled once: the message id is stored on the notification (`event_id`, unique), so a redelivery changes nothing and sends no second email.
- A handler error (the notification or the alert settings could not be read or stored) asks the broker for one more delivery; a second failure drops the message. An email that cannot leave is logged and never asks for a redelivery.
- The consumer reconnects on its own (1 s to 30 s between attempts) and stops with the HTTP server. Events published while the service is down wait in the queue.
- Broker tests: `RABBITMQ_TEST_URL=amqp://guest:guest@localhost:5672/ go test -tags integration ./core/` (skipped without the variable).

## Alerts and emails

`alert_preferences` keeps, per user and kind (`applications`, `matches`, `closing`, `tips`), whether the alert goes by `email` and `in_app`. A kind the user never chose for keeps its default: both, except `tips` (in 9itchi only). `closing` and `tips` have no sender yet.

Emails are written in the user's language (`language` from account-api: `fr`, `en` or `ar`, French when unknown), all with the same layout: logo, category, heading, a short text, one button to `FRONTEND_URL`, and a footer leading to `/settings/alerts`. Arabic reads right to left. Every value is HTML-escaped. The address is fetched from account-api for each send and never stored or logged. Without Mailgun no email leaves and nothing else changes.

## Email channel (Mailgun)

Optional. When `MAILGUN_API_KEY` and `MAILGUN_DOMAIN` are set, an `email` channel is registered in `core.ChannelFactory`. It is opt-in per request: send `"channels": ["email"]` to `POST /internal/notifications` to also e-mail that notification (in-app always applies; unknown channels get 400; duplicates are delivered once; max 5). Subject = `title`, body = `data.body` or `data.message`, HTML-escaped. The recipient address is fetched from account-api (`GET /internal/users/:id`) and never stored or logged. Without both variables the factory only has `in_app`. Up to 3 attempts (500ms x attempt backoff), retried only on Mailgun 5xx or network errors; a failure is logged and never blocks the stored notification. Delivery is synchronous but capped at 20s (8s per attempt), after the live Ably push.

| Variable | Default | Meaning |
|---|---|---|
| `MAILGUN_API_KEY` | empty | Mailgun private API key (secret) |
| `MAILGUN_DOMAIN` | empty | Sending domain |
| `MAILGUN_FROM_EMAIL` | `9itchi <no-reply@DOMAIN>` | Sender |
| `MAILGUN_API_BASE` | empty (US) | `https://api.eu.mailgun.net` for the EU region |
| `MAILGUN_RATE_LIMIT` | `10` | Max e-mails per second |

## Run

```
cp .env.example .env
go run main.go
```
