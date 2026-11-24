package core

import (
	"html"
	"strings"
)

// EmailContent is one email 9itchi sends: a short text and one button, the same layout for all of them.
type EmailContent struct {
	// Lang is fr, en or ar: it sets the direction of the page.
	Lang string
	// Subject is the subject line and the heading; Preheader is the grey line mail clients show after it.
	Subject   string
	Preheader string
	// Category is the small label beside the logo (« Suivi », « Paiement »…).
	Category string
	Text     string
	// Button and URL are the one action of the email. Footer is the line saying why it was received, FooterLink
	// and FooterURL the way to the alert settings (empty: no link).
	Button     string
	URL        string
	Footer     string
	FooterLink string
	FooterURL  string
}

// RenderEmail writes the plain-text and HTML bodies of an email. Every value is HTML-escaped; URLs must be built by
// the caller from its own configuration, never from what a user typed.
func RenderEmail(c EmailContent) (text, htmlBody string) {
	var t strings.Builder
	t.WriteString(c.Text)
	if c.URL != "" {
		t.WriteString("\n\n" + c.Button + " : " + c.URL)
	}
	if c.Footer != "" {
		t.WriteString("\n\n" + c.Footer)
		if c.FooterURL != "" {
			t.WriteString(" " + c.FooterLink + " : " + c.FooterURL)
		}
	}

	dir, align, opposite := "ltr", "left", "right"
	if c.Lang == "ar" {
		dir, align, opposite = "rtl", "right", "left"
	}
	e := html.EscapeString
	paragraphs := strings.ReplaceAll(e(c.Text), "\n", "<br>")
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="` + e(c.Lang) + `" dir="` + dir + `"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>` + e(c.Subject) + `</title></head>`)
	b.WriteString(`<body style="margin:0;padding:0;background:#FAFAF9;">`)
	if c.Preheader != "" {
		b.WriteString(`<div style="display:none;max-height:0;overflow:hidden;opacity:0;">` + e(c.Preheader) + `</div>`)
	}
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#FAFAF9;"><tr><td align="center" style="padding:24px 12px;">`)
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" dir="` + dir + `" style="max-width:520px;background:#ffffff;border:1px solid #ECEEF1;border-radius:18px;font-family:Inter,Arial,Helvetica,sans-serif;font-size:14px;line-height:1.6;color:#344054;text-align:` + align + `;"><tr><td style="padding:20px;">`)
	// Logo and category.
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr>`)
	b.WriteString(`<td style="text-align:` + align + `;"><span style="display:inline-block;width:22px;height:22px;line-height:22px;border-radius:6px;background:#D6F25E;text-align:center;font-weight:800;font-size:13px;color:#101218;">9</span> <strong style="color:#101218;">9itchi</strong></td>`)
	if c.Category != "" {
		b.WriteString(`<td style="text-align:` + opposite + `;"><span style="display:inline-block;height:24px;line-height:24px;padding:0 9px;border-radius:999px;background:#F2F3F5;color:#344054;font-size:12px;font-weight:600;">` + e(c.Category) + `</span></td>`)
	}
	b.WriteString(`</tr></table>`)
	b.WriteString(`<h1 style="margin:16px 0 0;font-size:18px;line-height:1.35;color:#101218;">` + e(c.Subject) + `</h1>`)
	b.WriteString(`<p style="margin:12px 0 0;">` + paragraphs + `</p>`)
	if c.URL != "" {
		b.WriteString(`<p style="margin:16px 0 0;"><a href="` + e(c.URL) + `" style="display:inline-block;height:40px;line-height:40px;padding:0 16px;border-radius:10px;background:#101218;color:#ffffff;font-weight:600;text-decoration:none;">` + e(c.Button) + `</a></p>`)
	}
	if c.Footer != "" {
		b.WriteString(`<p style="margin:16px 0 0;font-size:11.5px;color:#98A2B3;">` + e(c.Footer))
		if c.FooterURL != "" {
			b.WriteString(` <a href="` + e(c.FooterURL) + `" style="color:#98A2B3;">` + e(c.FooterLink) + `</a>`)
		}
		b.WriteString(`</p>`)
	}
	b.WriteString(`</td></tr></table></td></tr></table></body></html>`)
	return t.String(), b.String()
}
