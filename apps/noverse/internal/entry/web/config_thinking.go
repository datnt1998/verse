package web

import (
	"fmt"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/core/llm"
	"github.com/datnt1998/verse/core/roles"
)

// AU2: the eight-label vocabulary is offered by presets, but a saved level must
// be supported by the selected model. Reject only changed selections so a
// legacy config with an unsupported leftover value can still save unrelated
// settings (load-time Resolve clamps it to Tự động).
func validateThinkingSelections(saved, next bootstrap.Config, hasSaved bool) string {
	if !hasSaved || saved.Provider != next.Provider || saved.ModelName != next.ModelName || saved.ReasoningEffort != next.ReasoningEffort {
		if msg := checkThinking(next, next.Provider, next.ModelName, next.ReasoningEffort); msg != "" {
			return msg
		}
	}
	names := make([]string, 0, len(next.Roles))
	for role := range next.Roles {
		names = append(names, role)
	}
	for _, role := range names {
		rc := next.Roles[role]
		old, hadOld := saved.Roles[role]
		if hasSaved && hadOld &&
			old.Provider == rc.Provider && old.Model == rc.Model && old.ReasoningEffort == rc.ReasoningEffort {
			continue
		}
		provider, model, _ := roles.Resolve(next.Roles, role, next.Provider, next.ModelName)
		if msg := checkThinking(next, provider, model, rc.ReasoningEffort); msg != "" {
			return msg
		}
	}
	return ""
}

// validateFallbackProviders enforces the artboard rule "Dự phòng phải ở nhà
// cung cấp khác": a fallback on the same provider cannot survive that
// provider's key/quota failure, so it would only repeat the same error.
func validateFallbackProviders(saved, next bootstrap.Config, hasSaved bool) string {
	for role, rc := range next.Roles {
		old, hadOld := saved.Roles[role]
		if hasSaved && hadOld && old.Provider == rc.Provider && old.Model == rc.Model && sameModelRefs(old.Fallbacks, rc.Fallbacks) {
			continue
		}
		primary, _, _ := roles.Resolve(next.Roles, role, next.Provider, next.ModelName)
		for _, fallback := range rc.Fallbacks {
			if strings.TrimSpace(fallback.Provider) == "" || strings.TrimSpace(fallback.Model) == "" {
				return fmt.Sprintf("Dự phòng của %s thiếu nhà cung cấp hoặc model", role)
			}
			if fallback.Provider == primary {
				return fmt.Sprintf("Dự phòng của %s phải ở nhà cung cấp khác (%s)", role, primary)
			}
		}
	}
	return ""
}

func sameModelRefs(a, b []roles.ModelRef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func checkThinking(cfg bootstrap.Config, provider, model, level string) string {
	level = strings.TrimSpace(level)
	if level == "" || provider == "" || model == "" {
		return ""
	}
	lv, err := llm.ParseThinkingLevel(level)
	if err != nil {
		return err.Error()
	}
	probe, err := cfg.ProbeModel(provider, model)
	if err != nil {
		return "" // provider/model validation reports the real problem
	}
	if !llm.ThinkingPolicyFor(probe).Allows(lv) {
		return fmt.Sprintf("Mức suy luận %q không được model %s/%s hỗ trợ", level, provider, model)
	}
	return ""
}
