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
	// Rows are the facts the email is about (company, offer, when), shown as a summary card under the text. Empty
	// rows are not given by the caller.
	Rows []EmailRow
	// Button and URL are the one action of the email. Footer is the line saying why it was received, FooterLink
	// and FooterURL the way to the alert settings (empty: no link).
	Button     string
	URL        string
	Footer     string
	FooterLink string
	FooterURL  string
}

// EmailRow is one line of the summary card of an email: a label and its value.
type EmailRow struct{ Label, Value string }

// Fonts of the app (tailwind.config.ts): Bricolage Grotesque for headings, Inter for text, Noto Sans Arabic for
// Arabic. Mail clients that load no web font fall back to Arial.
const (
	emailFontText    = "Inter,'Noto Sans Arabic',Arial,Helvetica,sans-serif"
	emailFontDisplay = "'Bricolage Grotesque',Inter,'Noto Sans Arabic',Arial,Helvetica,sans-serif"
	emailFontsCSS    = "https://fonts.googleapis.com/css2?family=Bricolage+Grotesque:wght@700;800&family=Inter:wght@400;600;700&family=Noto+Sans+Arabic:wght@400;600;700&display=swap"
)

// RenderEmail writes the plain-text and HTML bodies of an email. Every value is HTML-escaped; URLs must be built by
// the caller from its own configuration, never from what a user typed.
func RenderEmail(c EmailContent) (text, htmlBody string) {
	var t strings.Builder
	t.WriteString(c.Text)
	if len(c.Rows) > 0 {
		t.WriteString("\n")
		for _, r := range c.Rows {
			t.WriteString("\n" + r.Label + " : " + r.Value)
		}
	}
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
	b.WriteString(`<!doctype html><html lang="` + e(c.Lang) + `" dir="` + dir + `"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>` + e(c.Subject) + `</title><link rel="stylesheet" href="` + emailFontsCSS + `"></head>`)
	b.WriteString(`<body style="margin:0;padding:0;background:#FAFAF9;">`)
	if c.Preheader != "" {
		b.WriteString(`<div style="display:none;max-height:0;overflow:hidden;opacity:0;">` + e(c.Preheader) + `</div>`)
	}
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#FAFAF9;"><tr><td align="center" style="padding:24px 12px;">`)
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" dir="` + dir + `" style="max-width:520px;background:#ffffff;border:1.5px solid #101218;border-radius:18px;box-shadow:5px 5px 0 #101218;font-family:` + emailFontText + `;font-size:14px;line-height:1.6;color:#344054;text-align:` + align + `;"><tr><td style="padding:24px;">`)
	// Logo and category.
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr>`)
	b.WriteString(`<td style="text-align:` + align + `;"><span style="display:inline-block;width:22px;height:22px;line-height:22px;border-radius:6px;background:#D6F25E;text-align:center;font-weight:800;font-size:13px;color:#101218;font-family:` + emailFontDisplay + `;">9</span> <strong style="color:#101218;font-family:` + emailFontDisplay + `;font-size:15px;">9itchi</strong></td>`)
	if c.Category != "" {
		b.WriteString(`<td style="text-align:` + opposite + `;"><span style="display:inline-block;height:24px;line-height:24px;padding:0 9px;border-radius:999px;background:#F2F3F5;color:#344054;font-size:12px;font-weight:600;">` + e(c.Category) + `</span></td>`)
	}
	b.WriteString(`</tr></table>`)
	b.WriteString(`<h1 style="margin:18px 0 0;font-family:` + emailFontDisplay + `;font-size:21px;font-weight:800;line-height:1.3;letter-spacing:-0.01em;color:#101218;">` + e(c.Subject) + `</h1>`)
	b.WriteString(`<p style="margin:12px 0 0;">` + paragraphs + `</p>`)
	if len(c.Rows) > 0 {
		b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:16px 0 0;background:#F7F8FA;border:1px solid #ECEEF1;border-radius:12px;">`)
		for i, r := range c.Rows {
			line := ""
			if i > 0 {
				line = "border-top:1px solid #ECEEF1;"
			}
			b.WriteString(`<tr><td style="padding:10px 14px;` + line + `font-size:12.5px;color:#596070;white-space:nowrap;">` + e(r.Label) + `</td>`)
			b.WriteString(`<td style="padding:10px 14px;` + line + `font-size:13.5px;font-weight:600;color:#101218;text-align:` + opposite + `;">` + e(r.Value) + `</td></tr>`)
		}
		b.WriteString(`</table>`)
	}
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
