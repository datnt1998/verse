package identity

import "errors"

// SessionEnded carries metadata only after the complete cookie secret matches.
// It never carries user data and remains compatible with ErrUnauthenticated.
type SessionEnded struct {
	Reason string
	App    App
}

func (e *SessionEnded) Error() string { return ErrUnauthenticated.Error() }
func (e *SessionEnded) Unwrap() error { return ErrUnauthenticated }

// SessionEndedBody is the shared 401 response, including for missing cookies.
type SessionEndedBody struct {
	Code   string `json:"code"`
	Error  string `json:"error"`
	Reason string `json:"reason"`
	App    App    `json:"app,omitempty"`
}

func EndedBody(err error) SessionEndedBody {
	b := SessionEndedBody{Code: "session_ended", Error: ErrUnauthenticated.Error(), Reason: "unknown"}
	var ended *SessionEnded
	if errors.As(err, &ended) {
		switch ended.Reason {
		case "logout", "password_changed", "admin_changed", "locked", "expired":
			b.Reason = ended.Reason
		}
		if b.Reason != "unknown" && ended.App.Valid() {
			b.App = ended.App
		}
	}
	return b
}

// ActorApp is request attribution, never inferred from the target grant.
func ActorApp(actor *AuditActor) App {
	if actor != nil && actor.App.Valid() {
		return actor.App
	}
	return ""
}
