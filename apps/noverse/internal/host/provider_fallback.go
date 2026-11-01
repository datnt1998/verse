package host

import (
	"fmt"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
)

// fallbackState remembers a runtime-only cross-provider switch. It is never
// written to the config file: renewing the plan or saving a new provider must
// put the primary model back, and an undo must only touch intended AI values.
type fallbackState struct {
	Role         string
	FromProvider string
	FromModel    string
	ToProvider   string
	ToModel      string
	Reason       string
	Since        time.Time
	NextProbeAt  time.Time
}

type fallbackSwap struct {
	Role         string
	FromProvider string
	FromModel    string
	ToProvider   string
	ToModel      string
	Reason       string
}

const fallbackProbeAfter = 5 * time.Minute

// activateProviderFallback switches role to the first configured fallback whose
// provider differs from the failing one (the artboards require a different
// provider). It returns false when no usable target exists, so the engine can
// pause with the classified message instead of looping.
func (h *Host) activateProviderFallback(role, badProvider, reason string) (fallbackSwap, bool) {
	h.mu.Lock()
	if h.models == nil {
		h.mu.Unlock()
		return fallbackSwap{}, false
	}
	currentProvider, currentModel, _ := h.models.CurrentSelection(role)
	if currentProvider == "" {
		currentProvider, currentModel, _ = h.models.CurrentSelection("default")
	}
	roleCfg := h.cfg.Roles[role]
	var chosen fallbackSwap
	for _, target := range roleCfg.Fallbacks {
		if target.Provider == "" || target.Model == "" {
			continue
		}
		if target.Provider == currentProvider || target.Provider == badProvider {
			continue
		}
		if err := h.models.Swap(role, target.Provider, target.Model); err != nil {
			continue
		}
		now := time.Now()
		if h.fallbacks == nil {
			h.fallbacks = map[string]fallbackState{}
		}
		h.fallbacks[role] = fallbackState{
			Role: role, FromProvider: currentProvider, FromModel: currentModel,
			ToProvider: target.Provider, ToModel: target.Model,
			Reason: reason, Since: now, NextProbeAt: now.Add(fallbackProbeAfter),
		}
		chosen = fallbackSwap{
			Role: role, FromProvider: currentProvider, FromModel: currentModel,
			ToProvider: target.Provider, ToModel: target.Model, Reason: reason,
		}
		h.lastFallback = &FallbackSnapshot{
			Active: true, Role: role, Provider: target.Provider, Model: target.Model,
			FromProvider: currentProvider, FromModel: currentModel, Reason: reason, Since: now,
		}
		h.applyThinkingLocked(role)
		h.mu.Unlock()
		h.emitFallbackActive(chosen)
		return chosen, true
	}
	h.mu.Unlock()
	return fallbackSwap{}, false
}

func (h *Host) emitFallbackActive(swap fallbackSwap) {
	h.emitEvent(Event{
		Time: time.Now(), Category: "SYSTEM", Kind: "fallback.active", Level: "warn",
		Role: swap.Role, Provider: swap.ToProvider, Model: swap.ToModel,
		FromProvider: swap.FromProvider, FromModel: swap.FromModel, Reason: swap.Reason,
		Summary: fmt.Sprintf("Đang viết bằng model dự phòng %s vì %s %s.",
			swap.ToModel, providerDisplayName(swap.FromProvider), providerFailureReasonLabel(swap.Reason)),
	})
}

func providerFailureReasonLabel(kind string) string {
	switch kind {
	case providerFailureExpired:
		return "đã hết hạn gói"
	case providerFailureKey:
		return "báo khoá không dùng được"
	case providerFailureQuota:
		return "tạm hết lượt"
	default:
		return "gặp lỗi"
	}
}

// probePrimaryFallback tries the original model again after a cooldown. If it
// is still failing the engine immediately re-activates the fallback, so a
// renewed subscription returns to the primary model without a manual switch.
func (h *Host) probePrimaryFallback(role string) {
	h.mu.Lock()
	state, ok := h.fallbacks[role]
	if !ok || time.Now().Before(state.NextProbeAt) || h.models == nil {
		h.mu.Unlock()
		return
	}
	if err := h.models.Swap(role, state.FromProvider, state.FromModel); err != nil {
		h.mu.Unlock()
		return
	}
	delete(h.fallbacks, role)
	if h.lastFallback != nil && h.lastFallback.Role == role {
		h.lastFallback = nil
	}
	h.applyThinkingLocked(role)
	h.mu.Unlock()
	h.emitEvent(Event{
		Time: time.Now(), Category: "SYSTEM", Kind: "fallback.cleared", Level: "info",
		Role: role, Provider: state.FromProvider, Model: state.FromModel,
		FromProvider: state.ToProvider, FromModel: state.ToModel, Reason: state.Reason,
		Summary: fmt.Sprintf("Đã thử lại model chính %s.", state.FromModel),
	})
}

// recordProviderFailure stores the classified failure for the settings strip
// and the snapshot. retryAt is zero for permanent kinds.
func (h *Host) recordProviderFailure(provider, role, kind, message string, retryAt time.Time) {
	if provider == "" {
		return
	}
	failure := bootstrap.ProviderFailure{Kind: kind, Message: message, At: time.Now(), Role: role}
	if !retryAt.IsZero() {
		t := retryAt
		failure.RetryAt = &t
	}
	bootstrap.RecordProviderFailure(provider, failure)

	snapshot := &ProviderErrorSnapshot{Provider: provider, Kind: kind, Message: message}
	if !retryAt.IsZero() {
		t := retryAt
		snapshot.RetryAt = &t
	}
	h.mu.Lock()
	h.providerError = snapshot
	h.mu.Unlock()
}

// clearProviderFailure removes the strip after a successful call.
func (h *Host) clearProviderFailure(provider string) {
	if provider == "" {
		return
	}
	bootstrap.ClearProviderFailure(provider)
	h.mu.Lock()
	if h.providerError != nil && h.providerError.Provider == provider {
		h.providerError = nil
	}
	h.mu.Unlock()
}

// fallbackSnapshot returns a copy for UISnapshot.
func (h *Host) fallbackSnapshot() *FallbackSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.lastFallback == nil {
		return nil
	}
	copy := *h.lastFallback
	return &copy
}

func (h *Host) providerErrorSnapshot() *ProviderErrorSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.providerError == nil {
		return nil
	}
	copy := *h.providerError
	if h.providerError.RetryAt != nil {
		t := *h.providerError.RetryAt
		copy.RetryAt = &t
	}
	return &copy
}

// providerRoleOfAgent maps a worker name to the role whose fallbacks apply.
func providerRoleOfAgent(agent string) string {
	agent = strings.TrimSpace(agent)
	if strings.HasPrefix(agent, "architect_") {
		return "architect"
	}
	return agent
}
