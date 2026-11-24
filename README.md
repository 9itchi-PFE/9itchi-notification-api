# 9itchi-notification-api

In-app notifications delivered over Ably, optional email/push channels. Consumes events.

Port `8087`, health `/health`. Notifications are stored in PostgreSQL. Live push and the token endpoint use Ably when `ABLY_API_KEY` is set; without it `core.Realtime` is a no-op and `GET /notifications/ably-token` returns `503` `{"error":"realtime unavailable"}`. A publish failure never blocks the stored notification.

## Endpoints

| Method | Path | Access | Handler |
|---|---|---|---|
| GET | `/notifications` | user | ListNotifications (`page`, `page_size`, `unread=true`; returns `unread_count`) |
| PUT | `/notifications/:id/read` | user | MarkRead (404 if not yours) |
| PUT | `/notifications/read-all` | user | MarkAllRead |
| GET | `/notifications/ably-token` | user | AblyToken (Ably TokenRequest, 30 min TTL, `clientId` = caller, subscribe only on `user:{id}`; 503 without `ABLY_API_KEY`) |
| POST | `/internal/notifications` | internal | SendNotification (`user_id`, `type`, `title`, `data`, optional `channels`) |

Types: `email_opened`, `scheduled_email_sent`, `email_failed`, `new_matching_jobs`, `admin_message`, `system`.

Events: `NotificationService.HandleEvent` maps `email.opened`, `email.sent`, `email.failed`, `jobs.matched` (payload needs `user_id`) to notifications. The RabbitMQ consumer is not wired yet.

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
