package queue

import "encoding/json"

// Persisted settings may outlive an earlier UI/schema. Read malformed values
// as defaults rather than making the entire scheduler unavailable.
func ruleNumber(raw json.RawMessage, fallback, lo, hi int) int {
	var value int
	if json.Unmarshal(raw, &value) != nil || value < lo || value > hi {
		return fallback
	}
	return value
}
