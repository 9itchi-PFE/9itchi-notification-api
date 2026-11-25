package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"9itchi-notification-api/core"
	"9itchi-notification-api/models/api"
	"9itchi-notification-api/models/db"
	"9itchi-notification-api/repositories"
	"9itchi-notification-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sentEmail struct{ to, subject, text, html string }

// outbox is a core.Mailer that keeps what it was asked to send, and can fail like Mailgun being down.
type outbox struct {
	sent []sentEmail
	err  error
}

func (o *outbox) Mail(_ context.Context, to, subject, text, htmlBody string) error {
	o.sent = append(o.sent, sentEmail{to, subject, text, htmlBody})
	return o.err
}

// alertEnv is a service with everything the alerts need, each part replaceable.
type alertEnv struct {
	svc     NotificationService
	repo    *mocks.NotificationRepository
	stored  []*db.Notification
	prefs   *mocks.PreferenceRepository
	mail    *outbox
	user    *Recipient
	userErr error
	context *EmailContext
	ctxErr  error
}

func newAlertEnv() *alertEnv {
	e := &alertEnv{
		prefs:   &mocks.PreferenceRepository{},
		mail:    &outbox{},
		user:    &Recipient{Email: "ines@example.com", Name: "Ines Mansour", Language: "fr"},
		context: &EmailContext{ApplicationID: "app-1", JobID: "job-1", JobTitle: "Portail RH", Company: "Acme"},
	}
	e.repo = &mocks.NotificationRepository{CreateFn: func(ctx context.Context, n *db.Notification) (*db.Notification, error) {
		e.stored = append(e.stored, n)
		return createEcho()(ctx, n)
	}}
	e.svc = NewNotificationService(e.repo, core.NewChannelFactory(), nil, nil, WithAlerts(AlertDeps{
		Prefs: e.prefs, Mailer: e.mail,
		Users:        func(context.Context, string) (*Recipient, error) { return e.user, e.userErr },
		EmailContext: func(context.Context, string) (*EmailContext, error) { return e.context, e.ctxErr },
		FrontendURL:  "https://app.example/",
	}))
	return e
}

func opened() map[string]any {
	return map[string]any{"user_id": uid, "email_id": "mail-1", core.EventIDKey: "evt-1"}
}

func TestOpenedApplicationIsToldInTheAppAndByEmail(t *testing.T) {
	e := newAlertEnv()
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventEmailOpened, opened()))

	require.Len(t, e.stored, 1)
	assert.Equal(t, "email_opened", e.stored[0].Type)
	assert.Equal(t, "Acme", e.stored[0].Data["company"], "the notification can name the company")
	assert.Equal(t, "Portail RH", e.stored[0].Data["job_title"])
	assert.Equal(t, "app-1", e.stored[0].Data["application_id"])

	require.Len(t, e.mail.sent, 1)
	m := e.mail.sent[0]
	assert.Equal(t, "ines@example.com", m.to)
	assert.Equal(t, "Acme a ouvert ta candidature", m.subject)
	assert.Contains(t, m.text, "« Portail RH »")
	assert.Contains(t, m.html, `href="https://app.example/applications"`)
	assert.Contains(t, m.html, `href="https://app.example/settings/alerts"`)
}

func TestEmailIsWrittenInTheLanguageOfTheUser(t *testing.T) {
	for lang, want := range map[string]string{
		"en": "Acme opened your application",
		"ar": "Acme فتحت ترشّحك",
		"":   "Acme a ouvert ta candidature", // an account from before the language was kept
		"de": "Acme a ouvert ta candidature",
	} {
		e := newAlertEnv()
		e.user.Language = lang
		require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventEmailOpened, opened()))
		require.Len(t, e.mail.sent, 1, lang)
		assert.Equal(t, want, e.mail.sent[0].subject, lang)
	}
	e := newAlertEnv()
	e.user.Language = "ar"
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventEmailOpened, opened()))
	assert.Contains(t, e.mail.sent[0].html, `dir="rtl"`)
}

func TestAlertSettingsDecideWhereItGoes(t *testing.T) {
	tests := []struct {
		name                 string
		email, inApp         bool
		wantStored, wantMail int
	}{
		{"both", true, true, 1, 1},
		{"in the app only", false, true, 1, 0},
		{"by email only", true, false, 0, 1},
		{"nowhere", false, false, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newAlertEnv()
			e.prefs.Rows = map[string]db.AlertPreference{core.AlertApplications: {Kind: core.AlertApplications, Email: tt.email, InApp: tt.inApp}}
			require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventEmailOpened, opened()))
			assert.Len(t, e.stored, tt.wantStored)
			assert.Len(t, e.mail.sent, tt.wantMail)
		})
	}
}

// An application that could not leave is not something to switch off.
func TestFailedScheduledApplicationIsAlwaysTold(t *testing.T) {
	e := newAlertEnv()
	e.prefs.Rows = map[string]db.AlertPreference{core.AlertApplications: {Kind: core.AlertApplications}}
	payload := map[string]any{"user_id": uid, "email_id": "mail-1", "scheduled": true}
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventEmailFailed, payload))
	require.Len(t, e.stored, 1)
	require.Len(t, e.mail.sent, 1)
	assert.Equal(t, "Ta candidature pour Acme n’est pas partie", e.mail.sent[0].subject)

	e = newAlertEnv()
	payload["scheduled"] = false
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventEmailFailed, payload))
	assert.Empty(t, e.stored, "a send the student clicked shows its failure on the screen")
	assert.Empty(t, e.mail.sent)
}

func TestScheduledApplicationSentEmail(t *testing.T) {
	e := newAlertEnv()
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventEmailSent, map[string]any{"user_id": uid, "email_id": "mail-1", "scheduled": true}))
	require.Len(t, e.mail.sent, 1)
	assert.Equal(t, "Ta candidature pour Acme est partie", e.mail.sent[0].subject)
}

func TestWithoutTheOfferTheEmailSaysItWithoutNames(t *testing.T) {
	for name, setup := range map[string]func(e *alertEnv){
		"sent for no application": func(e *alertEnv) { e.context = nil },
		"email-api unreachable":   func(e *alertEnv) { e.context, e.ctxErr = nil, errors.New("connection refused") },
	} {
		t.Run(name, func(t *testing.T) {
			e := newAlertEnv()
			setup(e)
			require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventEmailOpened, opened()))
			require.Len(t, e.stored, 1, "the notification is still stored")
			assert.NotContains(t, e.stored[0].Data, "company")
			require.Len(t, e.mail.sent, 1)
			assert.Equal(t, "Ta candidature a été ouverte", e.mail.sent[0].subject)
			assert.NotContains(t, e.mail.sent[0].text, "«")
		})
	}
}

func TestTheSameEventAgainSendsNoSecondEmail(t *testing.T) {
	e := newAlertEnv()
	e.repo.CreateFn = func(context.Context, *db.Notification) (*db.Notification, error) {
		return nil, repositories.ErrConflict
	}
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventEmailOpened, opened()))
	assert.Empty(t, e.mail.sent)
}

func TestAnEmailThatCannotLeaveNeverUndoesTheNotification(t *testing.T) {
	t.Run("Mailgun down", func(t *testing.T) {
		e := newAlertEnv()
		e.mail.err = errors.New("mailgun 503")
		require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventEmailOpened, opened()))
		assert.Len(t, e.stored, 1)
	})
	t.Run("account-api down", func(t *testing.T) {
		e := newAlertEnv()
		e.userErr = errors.New("connection refused")
		require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventEmailOpened, opened()))
		assert.Len(t, e.stored, 1)
		assert.Empty(t, e.mail.sent)
	})
	t.Run("a user without an address", func(t *testing.T) {
		e := newAlertEnv()
		e.user.Email = ""
		require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventEmailOpened, opened()))
		assert.Empty(t, e.mail.sent)
	})
	t.Run("settings unreadable: one more delivery, nothing told twice", func(t *testing.T) {
		e := newAlertEnv()
		e.prefs.Err = errors.New("database down")
		assert.Error(t, e.svc.HandleEvent(context.Background(), core.EventEmailOpened, opened()))
		assert.Empty(t, e.stored)
		assert.Empty(t, e.mail.sent)
	})
}

func TestReceiptEmail(t *testing.T) {
	payment := "55555555-5555-4555-8555-555555555555"
	payload := func() map[string]any {
		// Amounts arrive as JSON numbers.
		return map[string]any{"user_id": uid, "payment_id": payment, "amount_millimes": float64(30000), "receipt_no": "2026-000123", "plan_name": "Abonnement étudiant"}
	}

	e := newAlertEnv()
	// A receipt is not an alert: it leaves even with every alert switched off.
	e.prefs.Rows = map[string]db.AlertPreference{core.AlertApplications: {Kind: core.AlertApplications}}
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventPaymentCompleted, payload()))
	assert.Empty(t, e.stored, "the receipt is an email, not a notification")
	require.Len(t, e.mail.sent, 1)
	m := e.mail.sent[0]
	assert.Equal(t, "Ton reçu 9itchi · 30,000 TND", m.subject)
	assert.Contains(t, m.text, "n° 2026-000123")
	assert.Contains(t, m.html, `href="https://app.example/settings/billing/receipts/`+payment+`"`)

	e = newAlertEnv()
	e.user.Language = "en"
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventPaymentCompleted, payload()))
	assert.Equal(t, "Your 9itchi receipt · 30.000 TND", e.mail.sent[0].subject)

	t.Run("in Arabic the number keeps its order", func(t *testing.T) {
		e := newAlertEnv()
		e.user.Language = "ar"
		require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventPaymentCompleted, payload()))
		assert.Contains(t, e.mail.sent[0].text, "\u20662026-000123\u2069", "isolated as left-to-right text")
	})
	t.Run("without its number the receipt is still announced", func(t *testing.T) {
		e := newAlertEnv()
		p := payload()
		delete(p, "receipt_no")
		require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventPaymentCompleted, p))
		require.Len(t, e.mail.sent, 1)
		assert.NotContains(t, e.mail.sent[0].text, "n°")
	})
	t.Run("nothing paid, nothing to confirm", func(t *testing.T) {
		e := newAlertEnv()
		p := payload()
		p["amount_millimes"] = float64(0)
		require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventPaymentCompleted, p))
		assert.Empty(t, e.mail.sent)
	})
	t.Run("a payload without a valid payment is ignored", func(t *testing.T) {
		e := newAlertEnv()
		p := payload()
		p["payment_id"] = "../../admin"
		require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventPaymentCompleted, p))
		assert.Empty(t, e.mail.sent)
	})
}

func TestWelcomeEmail(t *testing.T) {
	e := newAlertEnv()
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventUserRegistered, map[string]any{"user_id": uid}))
	assert.Empty(t, e.stored)
	require.Len(t, e.mail.sent, 1)
	assert.Equal(t, "Bienvenue sur 9itchi, Ines !", e.mail.sent[0].subject)
	assert.Contains(t, e.mail.sent[0].html, `href="https://app.example/jobs"`)

	e = newAlertEnv()
	e.user.Name = "  "
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventUserRegistered, map[string]any{"user_id": uid}))
	assert.Equal(t, "Bienvenue sur 9itchi !", e.mail.sent[0].subject)

	e = newAlertEnv()
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventUserRegistered, map[string]any{"user_id": "nope"}))
	assert.Empty(t, e.mail.sent)
}

func TestWithoutMailgunNoEmailLeavesAndNothingBreaks(t *testing.T) {
	var stored int
	repo := &mocks.NotificationRepository{CreateFn: func(ctx context.Context, n *db.Notification) (*db.Notification, error) {
		stored++
		return createEcho()(ctx, n)
	}}
	svc := newSvc(repo, nil)
	require.NoError(t, svc.HandleEvent(context.Background(), core.EventEmailOpened, opened()))
	require.NoError(t, svc.HandleEvent(context.Background(), core.EventPaymentCompleted, map[string]any{"user_id": uid, "payment_id": nid, "amount_millimes": float64(1000)}))
	require.NoError(t, svc.HandleEvent(context.Background(), core.EventUserRegistered, map[string]any{"user_id": uid}))
	assert.Equal(t, 1, stored)
}

// Every email exists in the three languages, with nothing left to fill in.
func TestEmailCatalogIsComplete(t *testing.T) {
	d := emailDetails{Name: "Ines", Company: "Acme", JobTitle: "Portail RH", Millimes: 30000, ReceiptNo: "2026-000123", LinkID: "pay-1", Count: 3}
	d.Day = time.Date(2026, 12, 24, 9, 0, 0, 0, time.UTC)
	for _, kind := range []string{emailOpened, emailScheduledSent, emailFailed, emailMatches, emailClosing, emailTips, emailGift, emailReceipt, emailWelcome, emailRenewalDue, emailRenewalLapsed} {
		for _, lang := range []string{"fr", "en", "ar"} {
			for _, details := range []emailDetails{d, {Millimes: 30000, Day: d.Day, Count: 1, LinkID: "id-1"}} {
				c, ok := writeEmail(kind, lang, "https://app.example", details)
				require.True(t, ok, kind+" "+lang)
				for _, v := range []string{c.Subject, c.Category, c.Text, c.Button, c.Footer, c.FooterLink} {
					assert.NotEmpty(t, v, kind+" "+lang)
					assert.NotContains(t, v, "{", kind+" "+lang+": a placeholder was left")
				}
				assert.True(t, strings.HasPrefix(c.URL, "https://app.example/"), c.URL)
				assert.Equal(t, lang, c.Lang)
			}
		}
	}
	_, ok := writeEmail("unknown", "fr", "https://app.example", d)
	assert.False(t, ok)
	assert.Equal(t, "30,000", tnd(30000, "fr"))
	assert.Equal(t, "35.500", tnd(35500, "en"))
	assert.Equal(t, "0.005", tnd(5, "ar"))
}

func TestAlertPreferences(t *testing.T) {
	e := newAlertEnv()
	res, apiErr := e.svc.GetAlertPreferences(context.Background(), uid)
	require.Nil(t, apiErr)
	assert.Equal(t, []api.AlertPreferenceResponse{
		{Kind: "applications", Email: true, InApp: true},
		{Kind: "matches", Email: true, InApp: true},
		{Kind: "closing", Email: true, InApp: true},
		{Kind: "tips", Email: false, InApp: true},
	}, res.Items, "the defaults, in the order of the screen")

	yes, no := true, false
	res, apiErr = e.svc.UpdateAlertPreferences(context.Background(), uid, api.UpdateAlertPreferencesRequest{Items: []api.AlertPreference{
		{Kind: "matches", Email: &no, InApp: &yes},
		{Kind: "tips", Email: &yes, InApp: &no},
	}})
	require.Nil(t, apiErr)
	assert.Equal(t, api.AlertPreferenceResponse{Kind: "applications", Email: true, InApp: true}, res.Items[0], "a kind left out keeps its choice")
	assert.Equal(t, api.AlertPreferenceResponse{Kind: "matches", Email: false, InApp: true}, res.Items[1])
	assert.Equal(t, api.AlertPreferenceResponse{Kind: "tips", Email: true, InApp: false}, res.Items[3])

	bad := []api.UpdateAlertPreferencesRequest{
		{},
		{Items: []api.AlertPreference{{Kind: "payments", Email: &yes, InApp: &yes}}},
		{Items: []api.AlertPreference{{Kind: "tips", Email: &yes}}},
		{Items: []api.AlertPreference{{Kind: "tips", Email: &yes, InApp: &yes}, {Kind: "tips", Email: &no, InApp: &no}}},
	}
	for i, req := range bad {
		_, apiErr = e.svc.UpdateAlertPreferences(context.Background(), uid, req)
		require.NotNil(t, apiErr, i)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code, i)
	}
	_, apiErr = e.svc.GetAlertPreferences(context.Background(), "nope")
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)

	e.prefs.Err = errors.New("database down")
	_, apiErr = e.svc.GetAlertPreferences(context.Background(), uid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	_, apiErr = e.svc.UpdateAlertPreferences(context.Background(), uid, api.UpdateAlertPreferencesRequest{Items: []api.AlertPreference{{Kind: "tips", Email: &yes, InApp: &yes}}})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}

func TestRenewalReminderEmail(t *testing.T) {
	payload := func(stage string) map[string]any {
		return map[string]any{"user_id": uid, "subscription_id": "s1", "plan_name": "Abonnement étudiant", "price_millimes": float64(35000), "ends_at": "2026-12-24T09:00:00Z", "stage": stage}
	}

	e := newAlertEnv()
	// The reminder is what keeps a subscription going: no alert setting switches it off.
	e.prefs.Rows = map[string]db.AlertPreference{core.AlertApplications: {Kind: core.AlertApplications}}
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventSubscriptionRenewalDue, payload("due")))
	assert.Empty(t, e.stored, "an email, not a notification: the app has its own banner")
	require.Len(t, e.mail.sent, 1)
	m := e.mail.sent[0]
	assert.Equal(t, "Ton abonnement est à renouveler le 24 décembre 2026", m.subject)
	assert.Contains(t, m.text, "paie 35,000 TND sur Flouci")
	assert.Contains(t, m.text, "Rien n’est prélevé automatiquement")
	assert.Contains(t, m.html, `href="https://app.example/settings/billing"`)

	e = newAlertEnv()
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventSubscriptionRenewalDue, payload("lapsed")))
	require.Len(t, e.mail.sent, 1)
	assert.Equal(t, "Ton abonnement s’est terminé le 24 décembre 2026", e.mail.sent[0].subject)
	assert.Contains(t, e.mail.sent[0].text, "Il n’a pas été renouvelé")

	for lang, want := range map[string]string{"en": "Your subscription is due for renewal on 24 December 2026", "ar": "يجب تجديد اشتراكك قبل 24 ديسمبر 2026"} {
		e = newAlertEnv()
		e.user.Language = lang
		require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventSubscriptionRenewalDue, payload("due")))
		assert.Equal(t, want, e.mail.sent[0].subject)
	}

	t.Run("the day is the one in Tunis", func(t *testing.T) {
		e := newAlertEnv()
		p := payload("due")
		p["ends_at"] = "2026-12-23T23:30:00Z" // 00:30 on the 24th in Tunis
		require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventSubscriptionRenewalDue, p))
		assert.Contains(t, e.mail.sent[0].subject, "24 décembre 2026")
	})
	t.Run("a payload that cannot be told is ignored, never delivered again", func(t *testing.T) {
		for name, change := range map[string]func(p map[string]any){
			"no user":       func(p map[string]any) { p["user_id"] = "nope" },
			"unknown stage": func(p map[string]any) { p["stage"] = "soon" },
			"no day":        func(p map[string]any) { p["ends_at"] = "tomorrow" },
			"no price":      func(p map[string]any) { p["price_millimes"] = float64(0) },
		} {
			e := newAlertEnv()
			p := payload("due")
			change(p)
			assert.NoError(t, e.svc.HandleEvent(context.Background(), core.EventSubscriptionRenewalDue, p), name)
			assert.Empty(t, e.mail.sent, name)
		}
	})
}

// The three alerts that are not about an application: each has its own setting, its own words and its own page.
func TestMatchesClosingAndTipsAlerts(t *testing.T) {
	tests := []struct {
		name, event, kind string
		payload           map[string]any
		subject, url      string
		inText            []string
	}{
		{"new offers", core.EventJobsMatched, core.AlertMatches, map[string]any{"count": float64(4)},
			"4 nouvelles offres pour toi", "https://app.example/jobs", []string{"4 offres"}},
		{"one new offer", core.EventJobsMatched, core.AlertMatches, map[string]any{"count": float64(1)},
			"1 nouvelle offre pour toi", "https://app.example/jobs", []string{"Une offre"}},
		{"a saved offer closes", core.EventJobClosingSoon, core.AlertClosing,
			map[string]any{"job_id": nid, "job_title": "Portail RH", "company": "Acme", "closes_on": "2026-12-03", "days_left": float64(2)},
			"Acme : l’offre ferme le 3 décembre 2026", "https://app.example/jobs/" + nid, []string{"« Portail RH »", "3 décembre 2026"}},
		{"CV tips", core.EventCVTips, core.AlertTips, map[string]any{"resume_id": nid, "count": float64(3), "rule": "summary"},
			"3 points à corriger sur ton CV", "https://app.example/resumes/" + nid + "/check", []string{"3 points"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newAlertEnv()
			e.prefs.Rows = map[string]db.AlertPreference{tt.kind: {Kind: tt.kind, Email: true, InApp: true}}
			payload := map[string]any{"user_id": uid, core.EventIDKey: "evt-9"}
			for k, v := range tt.payload {
				payload[k] = v
			}
			require.NoError(t, e.svc.HandleEvent(context.Background(), tt.event, payload))
			require.Len(t, e.stored, 1)
			assert.Equal(t, tt.payload, e.stored[0].Data, "the client writes the line from the data")
			require.Len(t, e.mail.sent, 1)
			m := e.mail.sent[0]
			assert.Equal(t, tt.subject, m.subject)
			assert.Contains(t, m.html, `href="`+tt.url+`"`)
			for _, want := range tt.inText {
				assert.Contains(t, m.text, want)
			}

			// Switched off, it goes nowhere.
			e = newAlertEnv()
			e.prefs.Rows = map[string]db.AlertPreference{tt.kind: {Kind: tt.kind}}
			require.NoError(t, e.svc.HandleEvent(context.Background(), tt.event, payload))
			assert.Empty(t, e.stored)
			assert.Empty(t, e.mail.sent)
		})
	}

	// Never chosen: new offers and closing offers go both ways, the CV tips stay in the app.
	for event, wantMail := range map[string]int{core.EventJobsMatched: 1, core.EventJobClosingSoon: 1, core.EventCVTips: 0} {
		e := newAlertEnv()
		require.NoError(t, e.svc.HandleEvent(context.Background(), event, map[string]any{
			"user_id": uid, "count": float64(2), "job_id": nid, "closes_on": "2026-12-03", "resume_id": nid,
		}))
		assert.Len(t, e.stored, 1, event)
		assert.Len(t, e.mail.sent, wantMail, event)
	}
}

func TestAGivenMonthIsToldInTheAppAndByEmail(t *testing.T) {
	gift := func() map[string]any {
		return map[string]any{"user_id": uid, "gift_id": "g-1", "plan_name": "Abonnement étudiant", "days": float64(30),
			"ends_at": "2026-12-24T09:00:00Z", core.EventIDKey: "evt-gift"}
	}
	e := newAlertEnv()
	// Every alert switched off: a gift is still told.
	e.prefs.Rows = map[string]db.AlertPreference{}
	for _, kind := range core.AlertKinds {
		e.prefs.Rows[kind] = db.AlertPreference{Kind: kind}
	}
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventSubscriptionGifted, gift()))
	require.Len(t, e.stored, 1)
	assert.Equal(t, "month_gifted", e.stored[0].Type)
	assert.Equal(t, "2026-12-24T09:00:00Z", e.stored[0].Data["ends_at"])
	assert.NotContains(t, e.stored[0].Data, "user_id")
	require.Len(t, e.mail.sent, 1)
	assert.Equal(t, "Un mois offert par l’équipe 9itchi", e.mail.sent[0].subject)
	assert.Contains(t, e.mail.sent[0].text, "jusqu’au 24 décembre 2026")
	assert.Contains(t, e.mail.sent[0].text, "https://app.example/settings/billing")

	e = newAlertEnv()
	e.user.Language = "ar"
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventSubscriptionGifted, gift()))
	require.Len(t, e.mail.sent, 1)
	assert.Contains(t, e.mail.sent[0].text, "24 ديسمبر 2026")

	e = newAlertEnv()
	bad := gift()
	delete(bad, "ends_at")
	require.NoError(t, e.svc.HandleEvent(context.Background(), core.EventSubscriptionGifted, bad))
	assert.Empty(t, e.stored, "a gift that does not say until when is not told")
	assert.Empty(t, e.mail.sent)

	e = newAlertEnv()
	e.repo.CreateFn = func(context.Context, *db.Notification) (*db.Notification, error) { return nil, errors.New("db down") }
	assert.Error(t, e.svc.HandleEvent(context.Background(), core.EventSubscriptionGifted, gift()), "worth one more delivery")
	assert.Empty(t, e.mail.sent)
}
