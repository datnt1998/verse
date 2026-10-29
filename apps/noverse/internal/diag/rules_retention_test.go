package diag

import (
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

func TestRetentionDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name string
		rule RuleFunc
		edit func(*Snapshot)
		want int
	}{
		{"hook two allowed", HookTypeChain, func(s *Snapshot) { s.Outline[0].HookType = "de_doa_moi"; s.Outline[1].HookType = "de_doa_moi" }, 0},
		{"hook three warn", HookTypeChain, func(s *Snapshot) {
			for i := range 3 {
				s.Outline[i].HookType = "de_doa_moi"
			}
		}, 1},
		{"hook missing breaks chain", HookTypeChain, func(s *Snapshot) {
			s.Outline[0].HookType = "de_doa_moi"
			s.Outline[2].HookType = "de_doa_moi"
			s.Outline[3].HookType = "de_doa_moi"
		}, 0},
		{"twist lower inclusive", MidpointTwistPosition, func(s *Snapshot) { s.Outline[8].MidpointTwist = true }, 0},
		{"twist upper inclusive", MidpointTwistPosition, func(s *Snapshot) { s.Outline[11].MidpointTwist = true }, 0},
		{"twist too early", MidpointTwistPosition, func(s *Snapshot) { s.Outline[7].MidpointTwist = true }, 1},
		{"twist too late", MidpointTwistPosition, func(s *Snapshot) { s.Outline[12].MidpointTwist = true }, 1},
		{"twist missing on retention outline", MidpointTwistPosition, func(s *Snapshot) { s.Outline[0].HookType = "lat_thong_tin" }, 1},
		{"old outline skips twist", MidpointTwistPosition, func(*Snapshot) {}, 0},
		{"promise lower inclusive", PromiseScenePosition, func(s *Snapshot) { s.Outline[6].Promise = "da_tra: cô rời nhà trước mọi người" }, 0},
		{"promise upper inclusive", PromiseScenePosition, func(s *Snapshot) { s.Outline[11].Promise = "da_tra: cô nhận lại cửa tiệm" }, 0},
		{"promise early", PromiseScenePosition, func(s *Snapshot) { s.Outline[5].Promise = "da_tra: cảnh trả lời tiêu đề" }, 1},
		{"promise late", PromiseScenePosition, func(s *Snapshot) { s.Outline[12].Promise = "da_tra: cảnh trả lời tiêu đề" }, 1},
		{"promise countdown not fulfillment", PromiseScenePosition, func(s *Snapshot) { s.Outline[8].Promise = "đếm ngược tới phiên tòa" }, 1},
		{"promise empty marker not fulfillment", PromiseScenePosition, func(s *Snapshot) { s.Outline[8].Promise = "da_tra: " }, 1},
		{"promise first fulfillment wins", PromiseScenePosition, func(s *Snapshot) {
			s.Outline[1].Promise = "da_tra: cảnh tiêu đề"
			s.Outline[8].Promise = "da_tra: lời hứa thứ hai"
		}, 1},
		{"old outline skips promise", PromiseScenePosition, func(*Snapshot) {}, 0},
		{"unexpanded arc skips position", MidpointTwistPosition, func(s *Snapshot) {
			s.Outline[0].MidpointTwist = true
			s.Volumes = []domain.VolumeOutline{{Arcs: []domain.ArcOutline{{EstimatedChapters: 8}}}}
		}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Snapshot{Outline: make([]domain.OutlineEntry, 20)}
			for i := range s.Outline {
				s.Outline[i].Chapter = i + 1
			}
			tc.edit(&s)
			got := tc.rule(&s)
			if len(got) != tc.want {
				t.Fatalf("findings=%v; want %d", got, tc.want)
			}
			for _, finding := range got {
				if finding.Severity != SevWarning {
					t.Fatalf("severity=%s", finding.Severity)
				}
			}
		})
	}
}
