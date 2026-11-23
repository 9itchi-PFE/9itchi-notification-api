# 9itchi-notification-api

In-app notifications delivered over Ably, optional email/push channels. Consumes events.

Port `8087`, health `/health`. Notifications are stored in PostgreSQL. Ably (token endpoint and live push) is not implemented yet: `GET /notifications/ably-token` returns `501` and `core.Realtime` defaults to a no-op.

## Endpoints

| Method | Path | Access | Handler |
|---|---|---|---|
| GET | `/notifications` | user | ListNotifications (`page`, `page_size`, `unread=true`; returns `unread_count`) |
| PUT | `/notifications/:id/read` | user | MarkRead (404 if not yours) |
| PUT | `/notifications/read-all` | user | MarkAllRead |
| GET | `/notifications/ably-token` | user | AblyToken (not implemented) |
| POST | `/internal/notifications` | internal | SendNotification (`user_id`, `type`, `title`, `data`) |

Types: `email_opened`, `scheduled_email_sent`, `email_failed`, `new_matching_jobs`, `admin_message`, `system`.

Events: `NotificationService.HandleEvent` maps `email.opened`, `email.sent`, `email.failed`, `jobs.matched` (payload needs `user_id`) to notifications. The RabbitMQ consumer is not wired yet.

## Run

```
cp .env.example .env
go run main.go
```
