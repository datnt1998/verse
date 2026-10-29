package host

import (
	"strings"
	"time"
)

// NormalizeUserEvent projects old display wrappers into the current audit fields.
// The original summary remains intact; user-authored text is never translated.
func NormalizeUserEvent(ev Event) Event {
	if !strings.EqualFold(ev.Category, "user") && ev.Agent != "user" && ev.Agent != "me" {
		return ev
	}
	if ev.Command != "" {
		return ev
	}
	for _, prefix := range []struct{ tag, command string }{
		{"[Tiếp tục]", "continue"}, {"[继续]", "continue"},
		{"[Can thiệp]", "steer"}, {"[用户干预]", "steer"}, {"[干预]", "steer"},
	} {
		if text, ok := strings.CutPrefix(ev.Summary, prefix.tag); ok {
			ev.Command, ev.Input = prefix.command, strings.TrimSpace(text)
			return ev
		}
	}
	if strings.HasPrefix(ev.Summary, "/") {
		fields := strings.Fields(ev.Summary)
		if len(fields) > 0 {
			ev.Command = strings.TrimPrefix(fields[0], "/")
			ev.Input = strings.TrimSpace(strings.TrimPrefix(ev.Summary, fields[0]))
			if ev.Command == "command" {
				ev.Summary = ev.Input
				ev.Command = ""
				return NormalizeUserEvent(ev)
			}
		}
	}
	return ev
}

// DeduplicateUserEvents collapses only the two legacy records for one input.
// Repeated clicks (including equal text/time) stay separate: pairing is one-to-one,
// between an audit /input row and a host bracket row, never between like records.
func DeduplicateUserEvents(events []Event) []Event {
	out := make([]Event, 0, len(events))
	paired := make([]bool, len(events))
	for i, original := range events {
		if paired[i] {
			continue
		}
		ev := NormalizeUserEvent(original)
		source := legacyUserSource(original)
		best := -1
		distance := 5*time.Second + 1
		if source != "" {
			for j := i + 1; j < len(events); j++ {
				otherSource := legacyUserSource(events[j])
				if paired[j] || otherSource == "" || source == otherSource {
					continue
				}
				other := NormalizeUserEvent(events[j])
				if ev.Input != other.Input || ev.Time.IsZero() || other.Time.IsZero() || (ev.SessionID != "" && other.SessionID != "" && ev.SessionID != other.SessionID) {
					continue
				}
				if ev.Command != "input" && other.Command != "input" && ev.Command != other.Command {
					continue
				}
				delta := ev.Time.Sub(other.Time).Abs()
				if delta < distance {
					best, distance = j, delta
				}
			}
		}
		if best >= 0 {
			paired[best] = true
			other := NormalizeUserEvent(events[best])
			if source == "audit" {
				ev.Command = other.Command
			} else {
				other.Command = ev.Command
				ev = other // Authenticated audit identity wins over the machine USER.
			}
			ev.Summary = ev.Input
		}
		out = append(out, ev)
	}
	return out
}

func legacyUserSource(ev Event) string {
	if !strings.EqualFold(ev.Category, "user") && ev.Agent != "user" && ev.Agent != "me" {
		return ""
	}
	if strings.HasPrefix(ev.Summary, "[") {
		normal := NormalizeUserEvent(ev)
		if normal.Command == "continue" || normal.Command == "steer" {
			return "host"
		}
	}
	for _, prefix := range []string{"/input ", "/continue ", "/steer "} {
		if strings.HasPrefix(ev.Summary, prefix) {
			return "audit"
		}
	}
	return ""
}
