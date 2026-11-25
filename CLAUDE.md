# 9itchi Notification API
Consumes events, stores in-app notifications (Postgres), pushes them live over Ably, sends Mailgun emails (alerts, welcome, receipt, renewal) in fr/en/ar. Port 8087, DB `notifications`. Publishes nothing.

## Conventions (shared by every 9itchi Go service)
- `handlers/` thin (validate, authorize, call service, JSON) -> `services/` all logic, returns `(T, *models.ApiError)` -> `repositories/` only Postgres, plain errors (`ErrNotFound`, `ErrConflict`). Error body `{"error": "..."}`.
- DB calls use `repositories.QueryTimeout`; multi-table writes `Store.WithTx`.
- Schema: add `migrations/NNNN_name.sql` (goose, embedded, runs at startup). Never edit a merged migration.
- Auth: `Authorization: Bearer` AES-GCM-wrapped HS256 JWT (claims `id`, `email`, `role`, `typ`, `exp`; access only). `/internal/*` = `x-api-key` (`INTERNAL_API_KEY`), blocked by nginx.
- Events: exchange `9itchi.events` (topic, routing key = event name), names in `core/events.go`. Durable queue per consumer; handler error = one redelivery, then dropped. Empty `RABBITMQ_URL` = no-op consumer.
- Env only, validated in `utils/config.go`. Common: `PORT`, `ENVIRONMENT`, `DATABASE_URL`, `ITCHI_SECRET_KEY`, `ITCHI_ENCRYPTION_KEY`, `INTERNAL_API_KEY`, `INTERNAL_API_KEY_REQUIRED`. Never log tokens, passwords, PII.
- Tests: `{resource}_test.go`, testify, func-field fakes (no testify/mock), success + error paths. Repo tests: `//go:build integration`, embedded Postgres.
- Check: `gofmt -l .; go build ./... && go vet ./... && go test ./...`; `go test -tags integration -count=1 ./repositories/...`; `golangci-lint run`.

## Routes
- User: `GET /notifications`, `PUT /notifications/:id/read`, `PUT /notifications/read-all`, `GET /notifications/ably-token` (503 without Ably), `GET|PUT /notifications/preferences`.
- Internal: `POST /internal/notifications`, `DELETE /internal/users/:id`.

## Data, peers, events
- Tables: `notifications` (unique partial `event_id` = dedup), `alert_preferences`. Latest `0005_month_gifted.sql`. `notifications_type_check` constraint: a new notification type needs a migration.
- Peers: account `GetUser` (email, name, language), email `GetContext` (application, job title/company).
- Consumes (queue `notification-api.events`): `email.opened`, `email.sent` + `email.failed` (scheduled only), `jobs.matched`, `job.closing_soon`, `cv.tips`, `subscription.gifted`, `payment.completed` (receipt), `user.registered` (welcome), `subscription.renewal_due` (`due` | `lapsed`).
- Providers: `core.Realtime` = Ably if `ABLY_API_KEY` else no-op; `ChannelFactory`: `in_app` always, email (Mailgun) only if `MAILGUN_API_KEY` + `MAILGUN_DOMAIN`.
- Env: `RABBITMQ_URL`, `ABLY_API_KEY`, `MAILGUN_API_KEY`, `MAILGUN_DOMAIN`, `MAILGUN_FROM_EMAIL`, `MAILGUN_API_BASE` (empty = US, EU `https://api.eu.mailgun.net`), `MAILGUN_RATE_LIMIT` (10/s), `ACCOUNT_API_URL`, `EMAIL_API_URL`, `FRONTEND_URL`.

## Domain rules
- Alert kinds `applications`, `matches`, `closing`, `tips`, each with `email` + `in_app` toggles; all on by default except `tips` email.
- Type to kind: `email_opened`, `scheduled_email_sent` -> applications; `new_matching_jobs` -> matches; `saved_job_closing` -> closing; `cv_tip` -> tips.
- Always sent, ignoring prefs: `month_gifted`, scheduled `email_failed`, welcome, receipt, renewal reminder (no auto-charge).
- Email templates fr/en/ar, fallback fr, ar is RTL with localized month names.
- Ably token: subscribe-only on `user:{id}`, TTL 30 min. Ably failure never blocks the stored row.
- Pending: first real Mailgun send, real Ably check, Arabic proofreading.
