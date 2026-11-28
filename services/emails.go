package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"9itchi-notification-api/core"
)

// The emails 9itchi sends, by kind. The first six follow a notification type, so the alert settings decide whether
// they leave; the others are sent whatever the settings say (a month given, a receipt, a welcome, a renewal reminder).
const (
	emailOpened        = "email_opened"
	emailScheduledSent = "scheduled_email_sent"
	emailFailed        = "email_failed"
	emailMatches       = "new_matching_jobs"
	emailClosing       = "saved_job_closing"
	emailTips          = "cv_tip"
	emailGift          = "month_gifted"
	emailReceipt       = "receipt"
	emailWelcome       = "welcome"
	emailRenewalDue    = "renewal_due"
	emailRenewalLapsed = "renewal_lapsed"
	emailSupportReply  = "support_reply"
)

// emailWords are the words of one kind of email in one language. The With* forms are used when the company or the
// offer is known, the One forms when the email counts exactly one thing; {company}, {title}, {name}, {amount}, {no},
// {date} and {count} are replaced.
type emailWords struct {
	Category    string
	Subject     string
	SubjectWith string // with {company}
	SubjectOne  string
	Text        string
	TextWith    string // with {title}
	TextOne     string
	Button      string
	// path is the page of the app the button opens; {id} is replaced by what the email is about (a payment, an
	// offer, a CV).
	path string
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
	emailMatches: {
		"fr": {Category: "Nouvelles offres", Subject: "{count} nouvelles offres pour toi", SubjectOne: "1 nouvelle offre pour toi",
			Text:    "{count} offres qui correspondent à ce que tu cherches viennent d’être publiées.",
			TextOne: "Une offre qui correspond à ce que tu cherches vient d’être publiée.",
			Button:  "Voir mes offres", path: "/jobs"},
		"en": {Category: "New offers", Subject: "{count} new offers for you", SubjectOne: "1 new offer for you",
			Text:    "{count} offers that fit what you are looking for were just published.",
			TextOne: "An offer that fits what you are looking for was just published.",
			Button:  "See my offers", path: "/jobs"},
		"ar": {Category: "عروض جديدة", Subject: "عروض جديدة تناسبك: {count}", SubjectOne: "عرض جديد يناسبك",
			Text:    "نُشرت عروض جديدة تناسب ما تبحث عنه (العدد: {count}).",
			TextOne: "نُشر عرض جديد يناسب ما تبحث عنه.",
			Button:  "عرض عروضي", path: "/jobs"},
	},
	emailClosing: {
		"fr": {Category: "Offre enregistrée", Subject: "Une offre enregistrée ferme le {date}", SubjectWith: "{company} : l’offre ferme le {date}",
			Text:     "Une offre que tu as enregistrée ferme le {date}. Si elle t’intéresse toujours, c’est le moment de postuler.",
			TextWith: "L’offre « {title} » que tu as enregistrée ferme le {date}. Si elle t’intéresse toujours, c’est le moment de postuler.",
			Button:   "Voir l’offre", path: "/jobs/{id}"},
		"en": {Category: "Saved offer", Subject: "A saved offer closes on {date}", SubjectWith: "{company}: the offer closes on {date}",
			Text:     "An offer you saved closes on {date}. If you are still interested, now is the time to apply.",
			TextWith: "The offer “{title}” you saved closes on {date}. If you are still interested, now is the time to apply.",
			Button:   "See the offer", path: "/jobs/{id}"},
		"ar": {Category: "عرض محفوظ", Subject: "عرض محفوظ يُغلق يوم {date}", SubjectWith: "{company}: يُغلق العرض يوم {date}",
			Text:     "عرض حفظتَه يُغلق يوم {date}. إن كان ما زال يهمّك، فهذا وقت الترشّح.",
			TextWith: "العرض «{title}» الذي حفظتَه يُغلق يوم {date}. إن كان ما زال يهمّك، فهذا وقت الترشّح.",
			Button:   "عرض العرض", path: "/jobs/{id}"},
	},
	emailTips: {
		"fr": {Category: "Conseils CV", Subject: "{count} points à corriger sur ton CV", SubjectOne: "1 point à corriger sur ton CV",
			Text:    "La vérification de ton CV trouve encore {count} points à corriger. Ouvre-la pour les voir et les régler un par un.",
			TextOne: "La vérification de ton CV trouve encore un point à corriger. Ouvre-la pour le voir et le régler.",
			Button:  "Vérifier mon CV", path: "/resumes/{id}/check"},
		"en": {Category: "CV advice", Subject: "{count} points to correct on your CV", SubjectOne: "1 point to correct on your CV",
			Text:    "The check of your CV still finds {count} points to correct. Open it to see them and fix them one by one.",
			TextOne: "The check of your CV still finds one point to correct. Open it to see it and fix it.",
			Button:  "Check my CV", path: "/resumes/{id}/check"},
		"ar": {Category: "نصائح السيرة الذاتية", Subject: "نقاط للتصحيح في سيرتك الذاتية: {count}", SubjectOne: "نقطة واحدة للتصحيح في سيرتك الذاتية",
			Text:    "ما زال فحص سيرتك الذاتية يجد نقاطًا للتصحيح (العدد: {count}). افتحه لرؤيتها وتصحيحها واحدة تلو الأخرى.",
			TextOne: "ما زال فحص سيرتك الذاتية يجد نقطة واحدة للتصحيح. افتحه لرؤيتها وتصحيحها.",
			Button:  "فحص سيرتي الذاتية", path: "/resumes/{id}/check"},
	},
	emailGift: {
		"fr": {Category: "Abonnement", Subject: "Un mois offert par l’équipe 9itchi",
			Text:   "L’équipe 9itchi t’offre un mois : ton abonnement va maintenant jusqu’au {date}. Tu n’as rien à payer.",
			Button: "Voir mon abonnement", path: "/settings/billing"},
		"en": {Category: "Subscription", Subject: "A month given by the 9itchi team",
			Text:   "The 9itchi team gives you a month: your subscription now runs until {date}. There is nothing to pay.",
			Button: "See my subscription", path: "/settings/billing"},
		"ar": {Category: "الاشتراك", Subject: "شهر هدية من فريق 9itchi",
			Text:   "يهديك فريق 9itchi شهرًا: اشتراكك يمتد الآن حتى {date}. لا شيء عليك دفعه.",
			Button: "عرض اشتراكي", path: "/settings/billing"},
	},
	emailSupportReply: {
		"fr": {Category: "Aide", Subject: "L’équipe 9itchi t’a répondu", SubjectOne: "L’équipe 9itchi t’a répondu",
			Text:    "Tu as {count} nouveaux messages de l’équipe 9itchi dans ton chat d’aide.",
			TextOne: "Tu as un nouveau message de l’équipe 9itchi dans ton chat d’aide.",
			Button:  "Lire la réponse", path: "/support"},
		"en": {Category: "Help", Subject: "The 9itchi team replied", SubjectOne: "The 9itchi team replied",
			Text:    "You have {count} new messages from the 9itchi team in your help chat.",
			TextOne: "You have a new message from the 9itchi team in your help chat.",
			Button:  "Read the reply", path: "/support"},
		"ar": {Category: "المساعدة", Subject: "ردّ عليك فريق 9itchi", SubjectOne: "ردّ عليك فريق 9itchi",
			Text:    "لديك رسائل جديدة من فريق 9itchi في محادثة المساعدة (العدد: {count}).",
			TextOne: "لديك رسالة جديدة من فريق 9itchi في محادثة المساعدة.",
			Button:  "قراءة الرد", path: "/support"},
	},
	emailReceipt: {
		"fr": {Category: "Paiement", Subject: "Ton reçu 9itchi · {amount} TND",
			Text:     "Merci ! Ton paiement est confirmé. Ton reçu est dans ton compte, à télécharger en PDF.",
			TextWith: "Merci ! Ton paiement est confirmé. Ton reçu n° {no} est dans ton compte, à télécharger en PDF.",
			Button:   "Voir le reçu", path: "/settings/billing/receipts/{id}"},
		"en": {Category: "Payment", Subject: "Your 9itchi receipt · {amount} TND",
			Text:     "Thank you! Your payment is confirmed. Your receipt is in your account, ready to download as a PDF.",
			TextWith: "Thank you! Your payment is confirmed. Receipt no. {no} is in your account, ready to download as a PDF.",
			Button:   "See the receipt", path: "/settings/billing/receipts/{id}"},
		"ar": {Category: "الدفع", Subject: "وصلك من 9itchi · {amount} TND",
			Text:     "شكرًا! تم تأكيد دفعتك. وصلك موجود في حسابك وجاهز للتحميل بصيغة PDF.",
			TextWith: "شكرًا! تم تأكيد دفعتك. الوصل رقم {no} موجود في حسابك وجاهز للتحميل بصيغة PDF.",
			Button:   "عرض الوصل", path: "/settings/billing/receipts/{id}"},
	},
	emailRenewalDue: {
		"fr": {Category: "Abonnement", Subject: "Ton abonnement est à renouveler le {date}",
			Text:   "Ton abonnement se termine le {date}. Rien n’est prélevé automatiquement : paie {amount} TND sur Flouci pour garder la lettre, l’envoi en un clic et le suivi. Tu peux aussi l’arrêter au même endroit.",
			Button: "Renouveler", path: "/settings/billing"},
		"en": {Category: "Subscription", Subject: "Your subscription is due for renewal on {date}",
			Text:   "Your subscription ends on {date}. Nothing is charged automatically: pay {amount} TND on Flouci to keep the letter, the one-click send and the follow-up. You can also stop it from the same page.",
			Button: "Renew", path: "/settings/billing"},
		"ar": {Category: "الاشتراك", Subject: "يجب تجديد اشتراكك قبل {date}",
			Text:   "ينتهي اشتراكك يوم {date}. لا يُخصم أي مبلغ تلقائيًا: ادفع {amount} TND عبر Flouci للاحتفاظ بالرسالة والإرسال بنقرة واحدة والمتابعة. يمكنك أيضًا إيقافه من الصفحة نفسها.",
			Button: "تجديد", path: "/settings/billing"},
	},
	emailRenewalLapsed: {
		"fr": {Category: "Abonnement", Subject: "Ton abonnement s’est terminé le {date}",
			Text:   "Il n’a pas été renouvelé. Paie {amount} TND sur Flouci pour retrouver la lettre, l’envoi en un clic et le suivi. Tes candidatures et ton CV sont toujours là.",
			Button: "Renouveler", path: "/settings/billing"},
		"en": {Category: "Subscription", Subject: "Your subscription ended on {date}",
			Text:   "It was not renewed. Pay {amount} TND on Flouci to get back the letter, the one-click send and the follow-up. Your applications and your CV are still there.",
			Button: "Renew", path: "/settings/billing"},
		"ar": {Category: "الاشتراك", Subject: "انتهى اشتراكك يوم {date}",
			Text:   "لم يُجدَّد. ادفع {amount} TND عبر Flouci لاستعادة الرسالة والإرسال بنقرة واحدة والمتابعة. ترشّحاتك وسيرتك الذاتية ما زالت محفوظة.",
			Button: "تجديد", path: "/settings/billing"},
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

// openedRows label the summary card of an opened application: the company, the offer and when it was opened. At
// is the word between the day and the hour.
var openedRows = map[string]struct{ Company, Offer, Opened, At string }{
	"fr": {"Entreprise", "Offre", "Ouverte le", "à"},
	"en": {"Company", "Offer", "Opened on", "at"},
	"ar": {"الشركة", "العرض", "فُتح يوم", "على الساعة"},
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
	// LinkID is what the button opens: the payment of a receipt, the offer that closes, the CV to check.
	LinkID string
	// Day is the day a subscription ends or ended (renewal reminders) or an offer closes; written for the language
	// of the email.
	Day time.Time
	// Count is how many offers are new, or how many points of a CV are left to correct.
	Count int
	// OpenedAt is when a recruiter opened the application (email_opened); written in Tunis time.
	OpenedAt time.Time
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
		if d.Count == 1 && w.SubjectOne != "" {
			subject, text = w.SubjectOne, w.TextOne
		}
	}
	no := d.ReceiptNo
	if lang == "ar" && no != "" {
		// A receipt number reads left to right; alone in Arabic text its two halves would swap around the hyphen.
		no = "\u2066" + no + "\u2069"
	}
	fill := strings.NewReplacer("{company}", d.Company, "{title}", d.JobTitle, "{name}", d.Name, "{amount}", tnd(d.Millimes, lang), "{no}", no, "{date}", longDate(d.Day, lang), "{count}", strconv.Itoa(d.Count))
	base := strings.TrimRight(frontendURL, "/")
	url := base + strings.ReplaceAll(w.path, "{id}", d.LinkID)
	footer := emailFooters[lang]
	var rows []core.EmailRow
	if kind == emailOpened {
		rows = openedSummary(lang, d)
	}
	return core.EmailContent{
		Lang: lang, Subject: fill.Replace(subject), Category: w.Category, Text: fill.Replace(text), Rows: rows,
		Button: w.Button, URL: url,
		Footer: footer.Text, FooterLink: footer.Link, FooterURL: base + "/settings/alerts",
	}, true
}

// openedSummary is the card of an opened application: only what is known is shown.
func openedSummary(lang string, d emailDetails) []core.EmailRow {
	l := openedRows[lang]
	var rows []core.EmailRow
	if d.Company != "" {
		rows = append(rows, core.EmailRow{Label: l.Company, Value: d.Company})
	}
	if d.JobTitle != "" {
		rows = append(rows, core.EmailRow{Label: l.Offer, Value: d.JobTitle})
	}
	if !d.OpenedAt.IsZero() {
		hour := d.OpenedAt.In(tunis).Format("15:04")
		if lang == "ar" {
			// A time reads left to right; isolated so the Arabic text does not reorder it.
			hour = "\u2066" + hour + "\u2069"
		}
		rows = append(rows, core.EmailRow{Label: l.Opened, Value: longDate(d.OpenedAt, lang) + " " + l.At + " " + hour})
	}
	return rows
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

// tunis is the time of the students: UTC+1 all year. A subscription that ends at 00:30 in Tunis ends that day, not
// the day before.
var tunis = time.FixedZone("Africa/Tunis", 3600)

var monthNames = map[string][12]string{
	"fr": {"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"},
	"en": {"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
	"ar": {"يناير", "فبراير", "مارس", "أبريل", "مايو", "يونيو", "يوليو", "أغسطس", "سبتمبر", "أكتوبر", "نوفمبر", "ديسمبر"},
}

// longDate writes a day in full for a language: 24 décembre 2026, 24 December 2026.
func longDate(t time.Time, lang string) string {
	if t.IsZero() {
		return ""
	}
	t = t.In(tunis)
	return fmt.Sprintf("%d %s %d", t.Day(), monthNames[emailLanguage(lang)][t.Month()-1], t.Year())
}
