package web

import (
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/toolsvc"
)

// installProgressWire là hình tiến độ cài dùng chung cho /api/tools và
// /api/setup — hai endpoint giữ đúng cùng một cấu trúc.
type installProgressWire struct {
	Phase string `json:"phase"`
	// PhaseLabel là câu pha tiếng Việt của Phase ("Đang tải mô hình nhận
	// giọng nói"): web hiện thẳng, không tự dịch enum.
	PhaseLabel string  `json:"phaseLabel,omitempty"`
	Detail     string  `json:"detail,omitempty"`
	BytesDone  int64   `json:"bytesDone"`
	BytesTotal *int64  `json:"bytesTotal"`
	Pct        *int    `json:"pct"`
	StartedAt  *string `json:"startedAt,omitempty"`
}

func installProgressToWire(p *domain.InstallProgress) *installProgressWire {
	if p == nil {
		return nil
	}
	out := &installProgressWire{
		Phase: p.Phase, PhaseLabel: toolsvc.PhaseLabel(p), Detail: p.Detail,
		BytesDone: p.BytesDone, BytesTotal: p.BytesTotal,
	}
	if p.BytesTotal != nil && *p.BytesTotal > 0 {
		pct := int(p.BytesDone * 100 / *p.BytesTotal)
		if pct > 100 {
			pct = 100
		}
		out.Pct = &pct
	}
	if p.StartedAt > 0 {
		at := time.Unix(p.StartedAt, 0).UTC().Format(time.RFC3339)
		out.StartedAt = &at
	}
	return out
}
