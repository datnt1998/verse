package host

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

const foundationPremise = `## Thể loại và giọng điệu
Trinh thám, tiết chế.
## Định vị thể loại
Điều tra trong một thị trấn.
## Xung đột cốt lõi
Sự thật đối lập với lòng trung thành.
## Mục tiêu nhân vật chính
Tìm người mất tích.
## Hướng kết cục
Công bố sự thật.
## Vùng cấm sáng tác
Không giải quyết bằng phép màu.
## Điểm bán hàng khác biệt
Nhân chứng không thể nói.
## Móc câu khác biệt
Lời khai nằm trong bản nhạc.
## Cam kết cốt lõi
Giải thích mọi manh mối.
## Tính phù hợp với truyện ngắn
Một vụ án trong một đêm.
`

func TestStoryReaderPremiseSectionsAndMissing(t *testing.T) {
	for _, tc := range []struct {
		name string
		tier domain.PlanningTier
		text string
		missing []string
	}{
		{"complete short", domain.PlanningTierShort, foundationPremise, []string{}},
		{"empty required section", domain.PlanningTierShort, strings.Replace(foundationPremise, "Tìm người mất tích.", "", 1), []string{"Mục tiêu nhân vật chính"}},
		{"legacy headings", domain.PlanningTierShort, strings.Replace(foundationPremise, "Thể loại và giọng điệu", "题材和基调", 1), []string{}},
		{"mid requirements", domain.PlanningTierMid, foundationPremise, []string{"Động cơ câu chuyện", "Chuyển hướng trung kỳ"}},
		{"long requirements", domain.PlanningTierLong, foundationPremise, []string{"Động cơ câu chuyện", "Tuyến quan hệ/trưởng thành", "Lộ trình nâng cấp", "Chuyển hướng trung kỳ", "Mệnh đề kết cục"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewStore(t.TempDir())
			if err := s.Init(); err != nil { t.Fatal(err) }
			if err := s.RunMeta.Save(domain.RunMeta{PlanningTier: tc.tier}); err != nil { t.Fatal(err) }
			if err := s.Outline.SavePremise(tc.text); err != nil { t.Fatal(err) }
			at := time.Date(2026, 10, 30, 12, 30, 0, 0, time.UTC)
			if err := os.Chtimes(filepath.Join(s.Dir(), "premise.md"), at, at); err != nil { t.Fatal(err) }
			got, err := NewStoryReaderWithStore(s).Premise()
			if err != nil { t.Fatal(err) }
			if !reflect.DeepEqual(got.Missing, tc.missing) { t.Fatalf("missing = %v, want %v", got.Missing, tc.missing) }
			if got.UpdatedAt == nil || !got.UpdatedAt.Equal(at) { t.Fatalf("updatedAt = %v, want %v", got.UpdatedAt, at) }
			if len(got.Sections) < 9 || got.Sections[0].Heading != "Thể loại và giọng điệu" || got.Sections[0].Text != "Trinh thám, tiết chế." { t.Fatalf("sections = %+v", got.Sections) }
		})
	}
}

func TestStoryReaderFoundationEmptyAndWorldFields(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil { t.Fatal(err) }
	r := NewStoryReaderWithStore(s)
	premise, err := r.Premise()
	if err != nil { t.Fatal(err) }
	if premise.Sections == nil || len(premise.Sections) != 0 || len(premise.Missing) != 9 || premise.UpdatedAt != nil { t.Fatalf("empty premise = %+v", premise) }
	rules, err := r.WorldRules()
	if err != nil { t.Fatal(err) }
	if rules == nil || len(rules) != 0 { t.Fatalf("empty world rules = %#v", rules) }
	want := []domain.WorldRule{{Category: "technology", Rule: "Máy chỉ nhận giọng nói.", Boundary: "Không nhận bản ghi."}}
	if err := s.World.SaveWorldRules(want); err != nil { t.Fatal(err) }
	rules, err = r.WorldRules()
	if err != nil { t.Fatal(err) }
	if !reflect.DeepEqual(rules, want) { t.Fatalf("world rules = %+v, want %+v", rules, want) }
}

func TestStoryReaderFoundationReadErrors(t *testing.T) {
	for _, name := range []string{"world_rules.json", "meta/run.json"} {
		t.Run(name, func(t *testing.T) {
			s := store.NewStore(t.TempDir())
			if err := s.Init(); err != nil { t.Fatal(err) }
			if err := os.WriteFile(filepath.Join(s.Dir(), name), []byte("{"), 0o644); err != nil { t.Fatal(err) }
			r := NewStoryReaderWithStore(s)
			var err error
			if name == "world_rules.json" { _, err = r.WorldRules() } else { _, err = r.Premise() }
			if err == nil { t.Fatalf("corrupt %s must propagate an error", name) }
		})
	}
}

func TestSnapshotFoundationMetadata(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil { t.Fatal(err) }
	h := &Host{cfg: bootstrap.Config{}, store: s, events: make(chan Event, 8)}
	h.observer = newObserver(s, h.emitEvent, h.emitDelta, h.emitClear, h.contextWindowForAgent)
	empty := h.Snapshot()
	data, err := json.Marshal(empty)
	if err != nil { t.Fatal(err) }
	if !strings.Contains(string(data), `"openThreads":[]`) || !strings.Contains(string(data), `"planningTier":""`) { t.Fatalf("empty snapshot = %s", data) }
	if err := s.RunMeta.Save(domain.RunMeta{PlanningTier: domain.PlanningTierMid}); err != nil { t.Fatal(err) }
	want := domain.StoryCompass{EndingDirection: "Trở về nhà.", EstimatedScale: "Hai tập.", OpenThreads: []string{"Người giữ chìa khoá.", "Món nợ cũ."}}
	if err := s.Outline.SaveCompass(want); err != nil { t.Fatal(err) }
	// Compass facts must not disappear for flat or not-yet-initialized progress.
	for _, layered := range []bool{false, true} {
		if err := s.Progress.Save(&domain.Progress{Layered: layered}); err != nil { t.Fatal(err) }
		snap := h.Snapshot()
		if snap.PlanningTier != domain.PlanningTierMid || snap.CompassDirection != want.EndingDirection || snap.CompassScale != want.EstimatedScale || !reflect.DeepEqual(snap.OpenThreads, want.OpenThreads) { t.Fatalf("layered=%v snapshot = %+v", layered, snap) }
	}
}
