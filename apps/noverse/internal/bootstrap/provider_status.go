package bootstrap

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ProviderFailure is the classified provider error the settings screen shows
// for one provider. AU3 keeps the three user-facing kinds the artboards use:
//
//   - expired: subscription/plan or prepaid balance is gone (HTTP 402,
//     insufficient_quota, plan expired) → user must renew/login.
//   - key: API key rejected (HTTP 401/403, invalid_api_key) → user must enter a
//     new key.
//   - quota: temporary rate/usage limit (HTTP 429, retry-after) → Noverse waits
//     and retries; beyond two minutes it falls back to another provider.
//
// The worker that observed the failure writes this file; the gateway reads it
// so Settings can show the same classified strip and a real countdown even
// though the two run as separate processes.
type ProviderFailure struct {
	Kind    string     `json:"kind"`
	Message string     `json:"message"`
	RetryAt *time.Time `json:"retryAt,omitempty"`
	At      time.Time  `json:"at"`
	Role    string     `json:"role,omitempty"`
}

const (
	ProviderFailureExpired = "expired"
	ProviderFailureKey     = "key"
	ProviderFailureQuota   = "quota"
)

// ProviderStatusPath returns ~/.ainovel/provider-status.json.
func ProviderStatusPath() string {
	dir := DefaultConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "provider-status.json")
}

var providerStatusMu sync.Mutex

// RecordProviderFailure stores the last classified failure for provider.
// Best-effort: a read-only HOME must not turn a provider error into a panic.
func RecordProviderFailure(provider string, failure ProviderFailure) {
	path := ProviderStatusPath()
	if path == "" || provider == "" || failure.Kind == "" {
		return
	}
	if failure.At.IsZero() {
		failure.At = time.Now()
	}
	providerStatusMu.Lock()
	defer providerStatusMu.Unlock()
	statuses := readProviderStatuses(path)
	statuses[provider] = failure
	writeProviderStatuses(path, statuses)
}

// ClearProviderFailure removes a provider entry after a successful call.
func ClearProviderFailure(provider string) {
	path := ProviderStatusPath()
	if path == "" || provider == "" {
		return
	}
	providerStatusMu.Lock()
	defer providerStatusMu.Unlock()
	statuses := readProviderStatuses(path)
	if _, ok := statuses[provider]; !ok {
		return
	}
	delete(statuses, provider)
	writeProviderStatuses(path, statuses)
}

// ProviderFailures returns the currently relevant classified failures.
// Quota entries expire shortly after their retry deadline; expired/key entries
// stay until the user fixes the provider or a call succeeds.
func ProviderFailures() map[string]ProviderFailure {
	path := ProviderStatusPath()
	if path == "" {
		return map[string]ProviderFailure{}
	}
	providerStatusMu.Lock()
	defer providerStatusMu.Unlock()
	statuses := readProviderStatuses(path)
	now := time.Now()
	out := make(map[string]ProviderFailure, len(statuses))
	for provider, failure := range statuses {
		if providerFailureStale(failure, now) {
			continue
		}
		out[provider] = failure
	}
	return out
}

func providerFailureStale(failure ProviderFailure, now time.Time) bool {
	switch failure.Kind {
	case ProviderFailureQuota:
		if failure.RetryAt != nil {
			return now.After(failure.RetryAt.Add(2 * time.Minute))
		}
		return now.Sub(failure.At) > 5*time.Minute
	case ProviderFailureExpired, ProviderFailureKey:
		return now.Sub(failure.At) > 24*time.Hour
	default:
		return now.Sub(failure.At) > 5*time.Minute
	}
}

func readProviderStatuses(path string) map[string]ProviderFailure {
	out := map[string]ProviderFailure{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]ProviderFailure{}
	}
	return out
}

func writeProviderStatuses(path string, statuses map[string]ProviderFailure) {
	if len(statuses) == 0 {
		_ = os.Remove(path)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(statuses, "", "  ")
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".provider-status-*.tmp")
	if err != nil {
		return
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	if err := os.Rename(tmpPath, path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return
	}
	committed = true
}
