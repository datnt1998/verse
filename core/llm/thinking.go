package llm

import "fmt"

var ThinkingLevelOrder = []ThinkingLevel{
	ThinkingOff,
	ThinkingMinimal,
	ThinkingLow,
	ThinkingMedium,
	ThinkingHigh,
	ThinkingXHigh,
	ThinkingMax,
}

type ThinkingPolicy struct {
	Available []ThinkingLevel
}

func ThinkingPolicyFor(model any) ThinkingPolicy {
	cp, ok := model.(CapabilityProvider)
	if !ok {
		return ThinkingPolicy{Available: append([]ThinkingLevel{ThinkingAuto}, ThinkingLevelOrder...)}
	}
	return ThinkingPolicyFromCapabilities(cp.Capabilities())
}

func ThinkingPolicyFromCapabilities(caps Capabilities) ThinkingPolicy {
	if !caps.ProviderBaseline {
		return exactThinkingPolicy(caps.Thinking)
	}
	return baselineThinkingPolicy(caps.Thinking)
}

func baselineThinkingPolicy(caps ThinkingCapabilities) ThinkingPolicy {
	if caps.Supported == SupportNo {
		return ThinkingPolicy{Available: []ThinkingLevel{ThinkingAuto}}
	}
	available := []ThinkingLevel{ThinkingAuto}
	for _, level := range ThinkingLevelOrder {
		if level == ThinkingOff && caps.Disable == SupportNo {
			continue
		}
		available = append(available, level)
	}
	return ThinkingPolicy{Available: available}
}

func exactThinkingPolicy(caps ThinkingCapabilities) ThinkingPolicy {
	if caps.Supported == SupportUnknown && caps.Disable == SupportUnknown && len(caps.Efforts) == 0 {
		return ThinkingPolicy{Available: append([]ThinkingLevel{ThinkingAuto}, ThinkingLevelOrder...)}
	}
	available := []ThinkingLevel{ThinkingAuto}
	if caps.Disable == SupportYes || caps.Disable == SupportPartial {
		available = append(available, ThinkingOff)
	}
	available = append(available, caps.Efforts...)
	return ThinkingPolicy{Available: uniqueThinkingLevels(available)}
}

func (p ThinkingPolicy) Allows(level ThinkingLevel) bool {
	for _, available := range p.Available {
		if available == level {
			return true
		}
	}
	return false
}

func (p ThinkingPolicy) Resolve(level ThinkingLevel) (ThinkingLevel, bool) {
	level = NormalizeThinkingLevel(level)
	if p.Allows(level) {
		return level, true
	}
	return ThinkingAuto, false
}

func uniqueThinkingLevels(levels []ThinkingLevel) []ThinkingLevel {
	seen := make(map[ThinkingLevel]struct{}, len(levels))
	out := make([]ThinkingLevel, 0, len(levels))
	for _, level := range levels {
		if _, ok := seen[level]; ok {
			continue
		}
		seen[level] = struct{}{}
		out = append(out, level)
	}
	return out
}

// ParseThinkingLevel chuyển chuỗi cấu hình thành ThinkingLevel.
// "" hợp lệ (= không ghi đè/kế thừa); các giá trị còn lại phải là
// off/minimal/low/medium/high/xhigh/max, sai thì trả lỗi (lúc khởi động hạ về
// rỗng kèm warn, lúc chạy lỗi được trả lại cho người dùng). "minimal" là mức
// Tối thiểu trong bộ tám nhãn của Gói AU; model không hỗ trợ vẫn bị
// ThinkingPolicy.Resolve kẹp về ThinkingAuto.
func ParseThinkingLevel(s string) (ThinkingLevel, error) {
	lv := NormalizeThinkingLevel(ThinkingLevel(s))
	switch lv {
	case ThinkingAuto, ThinkingOff, ThinkingMinimal, ThinkingLow, ThinkingMedium,
		ThinkingHigh, ThinkingXHigh, ThinkingMax:
		return lv, nil
	default:
		return "", fmt.Errorf("mức suy luận không hợp lệ %q (tùy chọn: off/minimal/low/medium/high/xhigh/max)", s)
	}
}
