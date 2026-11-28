package core

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderEmail(t *testing.T) {
	c := EmailContent{
		Lang: "fr", Subject: "Acme a ouvert ta candidature", Preheader: "Aujourd’hui", Category: "Suivi",
		Text: "Bonne nouvelle.\nÀ bientôt.", Button: "Voir mes candidatures", URL: "https://app.example/applications",
		Footer: "Tu reçois cet email car tu as un compte 9itchi.", FooterLink: "Gérer mes alertes", FooterURL: "https://app.example/settings/alerts",
	}
	text, html := RenderEmail(c)

	assert.Contains(t, text, "Bonne nouvelle.")
	assert.Contains(t, text, "Voir mes candidatures : https://app.example/applications")
	assert.Contains(t, text, "Gérer mes alertes : https://app.example/settings/alerts")

	assert.Contains(t, html, `<html lang="fr" dir="ltr">`)
	assert.Contains(t, html, `>Acme a ouvert ta candidature</h1>`)
	assert.Contains(t, html, "Bonne nouvelle.<br>À bientôt.")
	assert.Contains(t, html, `href="https://app.example/applications"`)
	assert.Contains(t, html, `href="https://app.example/settings/alerts"`)
	assert.Contains(t, html, ">Suivi</span>")
	assert.Equal(t, 1, strings.Count(html, "display:inline-block;height:40px"), "one button")
}

func TestRenderEmailSummaryCard(t *testing.T) {
	text, html := RenderEmail(EmailContent{Lang: "fr", Subject: "S", Text: "T", Rows: []EmailRow{{"Entreprise", "Acme"}, {"Offre", "<b>RH</b>"}}})
	assert.Contains(t, text, "\n\nEntreprise : Acme\nOffre : <b>RH</b>")
	assert.Contains(t, html, ">Entreprise</td>")
	assert.Contains(t, html, "&lt;b&gt;RH&lt;/b&gt;", "a value is escaped")
	assert.Contains(t, html, "Bricolage Grotesque", "the heading uses the display font of the app")
	assert.Contains(t, html, "box-shadow:5px 5px 0 #101218", "the card shadow of the app")

	_, html = RenderEmail(EmailContent{Lang: "fr", Subject: "S", Text: "T"})
	assert.NotContains(t, html, "#F7F8FA", "no card without rows")
}

func TestRenderEmailEscapesWhatAUserTyped(t *testing.T) {
	_, html := RenderEmail(EmailContent{
		Lang: "en", Subject: `<script>alert(1)</script> opened your application`, Category: `"><b>`,
		Text: `Your application “<img src=x onerror=alert(1)>” was opened.`, Button: "See", URL: `https://app.example/a?x="y"&z=1`,
	})
	assert.NotContains(t, html, "<script>")
	assert.NotContains(t, html, "<img")
	assert.NotContains(t, html, `"><b>`)
	assert.Contains(t, html, "&lt;script&gt;")
	assert.Contains(t, html, `href="https://app.example/a?x=&#34;y&#34;&amp;z=1"`)
}

func TestRenderEmailArabicReadsRightToLeft(t *testing.T) {
	_, html := RenderEmail(EmailContent{Lang: "ar", Subject: "تم فتح ترشّحك", Category: "المتابعة", Text: "خبر سار."})
	assert.Contains(t, html, `<html lang="ar" dir="rtl">`)
	assert.Contains(t, html, "text-align:right")
	assert.NotContains(t, html, "<a ", "no button and no footer link when none is given")
}

func TestAlertKinds(t *testing.T) {
	assert.Equal(t, []string{"applications", "matches", "closing", "tips"}, AlertKinds)
	assert.Equal(t, AlertChoice{Email: true, InApp: true}, DefaultAlert(AlertApplications))
	assert.Equal(t, AlertChoice{Email: false, InApp: true}, DefaultAlert(AlertTips), "tips come by email only when asked")
	assert.True(t, IsAlertKind("closing"))
	assert.False(t, IsAlertKind("payments"))
	assert.Equal(t, AlertApplications, AlertOfType("email_opened"))
	assert.Equal(t, AlertApplications, AlertOfType("scheduled_email_sent"))
	assert.Equal(t, AlertMatches, AlertOfType("new_matching_jobs"))
	assert.Empty(t, AlertOfType("email_failed"), "an application that could not leave is always told")
	assert.Empty(t, AlertOfType("system"))
}

func TestChannelFactoryMailer(t *testing.T) {
	f := NewChannelFactory()
	assert.Nil(t, f.Mailer(), "no Mailgun, no mailer")

	sender := &fakeSender{}
	ch := newTestChannel(sender, resolveTo("someone@example.com"))
	f[ChannelEmail] = func() Channel { return ch }
	m := f.Mailer()
	require.NotNil(t, m)

	require.NoError(t, m.Mail(context.Background(), "ines@example.com", "Subject\r\nBcc: x@example.com", "text", "<p>html</p>"))
	require.Len(t, sender.calls, 1)
	assert.Equal(t, "ines@example.com", sender.calls[0].to, "the address given, not the resolver's")
	assert.NotContains(t, sender.calls[0].subject, "\n", "no header injection through the subject")
	assert.Equal(t, "<p>html</p>", sender.calls[0].html)

	assert.Error(t, m.Mail(context.Background(), "", "s", "t", "h"), "no address, no email")
}
