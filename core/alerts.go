package core

// Alert kinds are the lines of « Paramètres → Alertes »: what a user can choose to be told, by email and in 9itchi.
const (
	AlertApplications = "applications" // an application was opened, a scheduled one left
	AlertMatches      = "matches"      // new offers for the user
	AlertClosing      = "closing"      // a saved offer closes soon
	AlertTips         = "tips"         // advice on the CV
)

// AlertKinds lists the kinds in the order the screen shows them.
var AlertKinds = []string{AlertApplications, AlertMatches, AlertClosing, AlertTips}

// AlertChoice says where a kind of alert goes.
type AlertChoice struct {
	Email bool
	InApp bool
}

// DefaultAlert is the choice of a user who never changed it: everything by both ways, except the tips by email.
func DefaultAlert(kind string) AlertChoice {
	return AlertChoice{Email: kind != AlertTips, InApp: true}
}

// IsAlertKind reports whether kind is one of AlertKinds.
func IsAlertKind(kind string) bool {
	for _, k := range AlertKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// alertOfType maps a notification type to the alert kind a user can switch off. A type that is not here is always
// delivered: a scheduled application that could not leave, a message from the team.
var alertOfType = map[string]string{
	"email_opened":         AlertApplications,
	"scheduled_email_sent": AlertApplications,
	"new_matching_jobs":    AlertMatches,
}

// AlertOfType returns the alert kind of a notification type, "" when the type is always delivered.
func AlertOfType(notifType string) string { return alertOfType[notifType] }
