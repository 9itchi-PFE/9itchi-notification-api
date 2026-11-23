# 9itchi-notification-api

In-app notifications delivered over Ably, optional email/push channels. Consumes events.

Port `8087`, health `/health`. Skeleton only: every endpoint returns `501 not implemented` until built.

## Endpoints

| Method | Path | Access | Handler |
|---|---|---|---|
| GET | `/notifications` | user | ListNotifications |
| PUT | `/notifications/:id/read` | user | MarkRead |
| PUT | `/notifications/read-all` | user | MarkAllRead |
| GET | `/notifications/ably-token` | user | AblyToken |
| POST | `/internal/notifications` | internal | SendNotification |

## Run

```
cp .env.example .env
go run main.go
```
