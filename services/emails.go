package services

import (
	"fmt"
	"strings"

	"9itchi-notification-api/core"
)

// The emails 9itchi sends, by kind. The first three follow a notification type; the last two are sent whatever the
// alert settings say (a receipt, a welcome).
const (
	emailOpened        = "email_opened"
	emailScheduledSent = "scheduled_email_sent"
	emailFailed        = "email_failed"
	emailReceipt       = "receipt"
	emailWelcome       = "welcome"
)

// emailWords are the words of one kind of email in one language. The With* forms are used when the company or the
// offer is known; {company}, {title}, {name}, {amount} and {no} are replaced.
type emailWords struct {
	Category       string
	Subject        string
	SubjectWith    string // with {company}
	Text           string
	TextWith       string // with {title}
	Button         string
	path           string // page of the app the button opens
	pathWithDetail bool   // the path ends with the payment id (receipt)
}

type footerWords struct{ Text, Link string }

var emailFooters = map[string]footerWords{
	"fr": {"Tu reçois cet email car tu as un compte 9itchi.", "Gérer mes alertes"},
	"en": {"You receive this email because you have a 9itchi account.", "Manage my alerts"},
	"ar": {"تصلك هذه الرسالة لأن لديك حسابًا على 9itchi.", "إدارة تنبيهاتي"},
}

var emailCatalog = map[string]map[string]emailWords{
	emailOpened: {
		"fr": {Category: "Suivi", Subject: "Ta candidature a été ouverte", SubjectWith: "{company} a ouvert ta candidature",
			Text:     "Bonne nouvelle : ta candidature a été ouverte. Les réponses arrivent souvent dans la semaine.",
			TextWith: "Bonne nouvelle : ta candidature « {title} » a été ouverte. Les réponses arrivent souvent dans la semaine.",
			Button:   "Voir mes candidatures", path: "/applications"},
		"en": {Category: "Follow-up", Subject: "Your application was opened", SubjectWith: "{company} opened your application",
			Text:     "Good news: your application was opened. Replies often come within the week.",
			TextWith: "Good news: your application “{title}” was opened. Replies often come within the week.",
			Button:   "See my applications", path: "/applications"},
		"ar": {Category: "المتابعة", Subject: "تم فتح ترشّحك", SubjectWith: "{company} فتحت ترشّحك",
			Text:     "خبر سار: تم فتح ترشّحك. الردود تصل غالبًا خلال الأسبوع.",
			TextWith: "خبر سار: تم فتح ترشّحك «{title}». الردود تصل غالبًا خلال الأسبوع.",
			Button:   "عرض ترشّحاتي", path: "/applications"},
	},
	emailScheduledSent: {
		"fr": {Category: "Envoi programmé", Subject: "Ta candidature programmée est partie", SubjectWith: "Ta candidature pour {company} est partie",
			Text:   "Comme prévu, ta candidature est partie de ton Gmail. On te prévient quand elle est ouverte.",
			Button: "Voir la candidature", path: "/applications"},
		"en": {Category: "Scheduled send", Subject: "Your scheduled application was sent", SubjectWith: "Your application to {company} was sent",
			Text:   "As planned, your application left from your Gmail. We will tell you when it is opened.",
			Button: "See the application", path: "/applications"},
		"ar": {Category: "إرسال مبرمج", Subject: "تم إرسال ترشّحك المبرمج", SubjectWith: "تم إرسال ترشّحك إلى {company}",
			Text:   "كما كان مقرّرًا، أُرسل ترشّحك من Gmail الخاص بك. سنخبرك عندما يُفتح.",
			Button: "عرض الترشّح", path: "/applications"},
	},
	emailFailed: {
		"fr": {Category: "Envoi programmé", Subject: "Ta candidature programmée n’est pas partie", SubjectWith: "Ta candidature pour {company} n’est pas partie",
			Text:   "L’envoi prévu n’a pas pu se faire. Ouvre tes candidatures pour voir ce qui s’est passé et la renvoyer.",
			Button: "Voir mes candidatures", path: "/applications"},
		"en": {Category: "Scheduled send", Subject: "Your scheduled application was not sent", SubjectWith: "Your application to {company} was not sent",
			Text:   "The planned send could not be done. Open your applications to see what happened and send it again.",
			Button: "See my applications", path: "/applications"},
		"ar": {Category: "إرسال مبرمج", Subject: "لم يُرسل ترشّحك المبرمج", SubjectWith: "لم يُرسل ترشّحك إلى {company}",
			Text:   "تعذّر الإرسال المبرمج. افتح ترشّحاتك لمعرفة ما حدث وإعادة الإرسال.",
			Button: "عرض ترشّحاتي", path: "/applications"},
	},
	emailReceipt: {
		"fr": {Category: "Paiement", Subject: "Ton reçu 9itchi · {amount} TND",
			Text:     "Merci ! Ton paiement est confirmé. Ton reçu est dans ton compte, à télécharger en PDF.",
			TextWith: "Merci ! Ton paiement est confirmé. Ton reçu n° {no} est dans ton compte, à télécharger en PDF.",
			Button:   "Voir le reçu", path: "/settings/billing/receipts/", pathWithDetail: true},
		"en": {Category: "Payment", Subject: "Your 9itchi receipt · {amount} TND",
			Text:     "Thank you! Your payment is confirmed. Your receipt is in your account, ready to download as a PDF.",
			TextWith: "Thank you! Your payment is confirmed. Receipt no. {no} is in your account, ready to download as a PDF.",
			Button:   "See the receipt", path: "/settings/billing/receipts/", pathWithDetail: true},
		"ar": {Category: "الدفع", Subject: "وصلك من 9itchi · {amount} TND",
			Text:     "شكرًا! تم تأكيد دفعتك. وصلك موجود في حسابك وجاهز للتحميل بصيغة PDF.",
			TextWith: "شكرًا! تم تأكيد دفعتك. الوصل رقم {no} موجود في حسابك وجاهز للتحميل بصيغة PDF.",
			Button:   "عرض الوصل", path: "/settings/billing/receipts/", pathWithDetail: true},
	},
	emailWelcome: {
		"fr": {Category: "Inscription", Subject: "Bienvenue sur 9itchi !", SubjectWith: "Bienvenue sur 9itchi, {name} !",
			Text:   "Ton compte est créé. Importe ton CV, corrige-le, puis postule aux offres faites pour toi.",
			Button: "Voir mes offres", path: "/jobs"},
		"en": {Category: "Sign-up", Subject: "Welcome to 9itchi!", SubjectWith: "Welcome to 9itchi, {name}!",
			Text:   "Your account is ready. Import your CV, fix it, then apply to the offers made for you.",
			Button: "See my offers", path: "/jobs"},
		"ar": {Category: "التسجيل", Subject: "مرحبًا بك في 9itchi!", SubjectWith: "مرحبًا بك في 9itchi يا {name}!",
			Text:   "تم إنشاء حسابك. استورد سيرتك الذاتية، صحّحها، ثم ترشّح للعروض المناسبة لك.",
			Button: "عرض عروضي", path: "/jobs"},
	},
}

// emailLanguage returns a language the catalog has: the user's, else French.
func emailLanguage(lang string) string {
	if _, ok := emailFooters[lang]; ok {
		return lang
	}
	return "fr"
}

// emailDetails is what an email can name. Every field may be empty: the email then says it without the name.
type emailDetails struct {
	Name      string // first name of the recipient (welcome)
	Company   string
	JobTitle  string
	Millimes  int64 // amount paid, in millimes (receipt); written for the language of the email
	ReceiptNo string
	PaymentID string
}

// writeEmail builds the email of a kind in a language. ok is false for a kind the catalog does not have.
func writeEmail(kind, lang, frontendURL string, d emailDetails) (c core.EmailContent, ok bool) {
	lang = emailLanguage(lang)
	w, ok := emailCatalog[kind][lang]
	if !ok {
		return c, false
	}
	subject, text := w.Subject, w.Text
	switch kind {
	case emailWelcome:
		if d.Name != "" {
			subject = w.SubjectWith
		}
	case emailReceipt:
		if d.ReceiptNo != "" {
			text = w.TextWith
		}
	default:
		if d.Company != "" && w.SubjectWith != "" {
			subject = w.SubjectWith
		}
		if d.JobTitle != "" && w.TextWith != "" {
			text = w.TextWith
		}
	}
	no := d.ReceiptNo
	if lang == "ar" && no != "" {
		// A receipt number reads left to right; alone in Arabic text its two halves would swap around the hyphen.
		no = "\u2066" + no + "\u2069"
	}
	fill := strings.NewReplacer("{company}", d.Company, "{title}", d.JobTitle, "{name}", d.Name, "{amount}", tnd(d.Millimes, lang), "{no}", no)
	base := strings.TrimRight(frontendURL, "/")
	url := base + w.path
	if w.pathWithDetail {
		url += d.PaymentID
	}
	footer := emailFooters[lang]
	return core.EmailContent{
		Lang: lang, Subject: fill.Replace(subject), Category: w.Category, Text: fill.Replace(text),
		Button: w.Button, URL: url,
		Footer: footer.Text, FooterLink: footer.Link, FooterURL: base + "/settings/alerts",
	}, true
}

// tnd writes millimes as dinars with three decimals, with the decimal sign of the language: 30,000 in French,
// 30.000 otherwise.
func tnd(millimes int64, lang string) string {
	sep := "."
	if lang == "fr" {
		sep = ","
	}
	return fmt.Sprintf("%d%s%03d", millimes/1000, sep, millimes%1000)
}

// firstName keeps the first word of a name, for a greeting.
func firstName(name string) string {
	if fields := strings.Fields(name); len(fields) > 0 {
		return fields[0]
	}
	return ""
}
