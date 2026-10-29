package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/errs"
)

// applyStep 是"在第 chapter 章应用一条 update"。
type applyStep struct {
	chapter int
	update  ForeshadowUpdate
}

// applyAll 用同一个 idx 连续应用一串操作，模拟"跨章重放整串记录"。
func applyAll(entries []ForeshadowEntry, idx map[string]int, steps []applyStep) ([]ForeshadowEntry, error) {
	var err error
	for _, step := range steps {
		entries, err = ApplyForeshadow(entries, idx, step.chapter, step.update)
		if err != nil {
			return entries, err
		}
	}
	return entries, nil
}

func TestApplyForeshadowTransitions(t *testing.T) {
	plant := func(id string, dueBy int) applyStep {
		return applyStep{1, ForeshadowUpdate{ID: id, Action: "plant", Description: "mô tả " + id, DueBy: dueBy}}
	}
	tests := []struct {
		name  string
		steps []applyStep
		want  ForeshadowEntry
	}{
		{
			name:  "plant ghi nhận due_by và last_touched",
			steps: []applyStep{plant("a", 5)},
			want:  ForeshadowEntry{ID: "a", Description: "mô tả a", PlantedAt: 1, Status: "planted", DueBy: 5, LastTouched: 1},
		},
		{
			name:  "plant cùng chương idempotent, giữ nguyên trạng thái đã đẩy",
			steps: []applyStep{plant("a", 0), {2, ForeshadowUpdate{ID: "a", Action: "advance"}}, plant("a", 0)},
			want:  ForeshadowEntry{ID: "a", Description: "mô tả a", PlantedAt: 1, Status: "advanced", LastTouched: 2},
		},
		{
			name:  "advance cập nhật last_touched",
			steps: []applyStep{plant("a", 0), {4, ForeshadowUpdate{ID: "a", Action: "advance"}}},
			want:  ForeshadowEntry{ID: "a", Description: "mô tả a", PlantedAt: 1, Status: "advanced", LastTouched: 4},
		},
		{
			name:  "resolve đóng phục bút tại chương",
			steps: []applyStep{plant("a", 0), {3, ForeshadowUpdate{ID: "a", Action: "resolve"}}},
			want:  ForeshadowEntry{ID: "a", Description: "mô tả a", PlantedAt: 1, Status: "resolved", ResolvedAt: 3, LastTouched: 1},
		},
		{
			name:  "abandon đóng phục bút kèm lý do",
			steps: []applyStep{plant("a", 0), {3, ForeshadowUpdate{ID: "a", Action: "abandon", Reason: "tuyến phụ bị cắt"}}},
			want: ForeshadowEntry{
				ID: "a", Description: "mô tả a", PlantedAt: 1, Status: "abandoned",
				LastTouched: 1, AbandonedAt: 3, AbandonReason: "tuyến phụ bị cắt",
			},
		},
		{
			name:  "advance sau resolve là no-op, không mở lại phục bút",
			steps: []applyStep{plant("a", 0), {3, ForeshadowUpdate{ID: "a", Action: "resolve"}}, {4, ForeshadowUpdate{ID: "a", Action: "advance"}}},
			want:  ForeshadowEntry{ID: "a", Description: "mô tả a", PlantedAt: 1, Status: "resolved", ResolvedAt: 3, LastTouched: 1},
		},
		{
			name:  "abandon sau resolve là no-op, giữ trạng thái đóng",
			steps: []applyStep{plant("a", 0), {3, ForeshadowUpdate{ID: "a", Action: "resolve"}}, {4, ForeshadowUpdate{ID: "a", Action: "abandon", Reason: "muộn"}}},
			want:  ForeshadowEntry{ID: "a", Description: "mô tả a", PlantedAt: 1, Status: "resolved", ResolvedAt: 3, LastTouched: 1},
		},
		{
			name:  "resolve lặp lại trong cùng chương idempotent",
			steps: []applyStep{plant("a", 0), {3, ForeshadowUpdate{ID: "a", Action: "resolve"}}, {3, ForeshadowUpdate{ID: "a", Action: "resolve"}}},
			want:  ForeshadowEntry{ID: "a", Description: "mô tả a", PlantedAt: 1, Status: "resolved", ResolvedAt: 3, LastTouched: 1},
		},
		{
			name:  "同章重复 plant 补上合法 due_by 仍幂等",
			steps: []applyStep{plant("a", 0), plant("a", 2)},
			want:  ForeshadowEntry{ID: "a", Description: "mô tả a", PlantedAt: 1, Status: "planted", DueBy: 2, LastTouched: 1},
		},
		{
			name:  "advance sau abandon không hồi sinh tuyến",
			steps: []applyStep{plant("a", 0), {2, ForeshadowUpdate{ID: "a", Action: "abandon", Reason: "bỏ"}}, {3, ForeshadowUpdate{ID: "a", Action: "advance"}}},
			want: ForeshadowEntry{
				ID: "a", Description: "mô tả a", PlantedAt: 1, Status: "abandoned",
				LastTouched: 1, AbandonedAt: 2, AbandonReason: "bỏ",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applyAll(nil, map[string]int{}, tc.steps)
			if err != nil {
				t.Fatalf("ApplyForeshadow: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("muốn 1 mục, nhận %+v", got)
			}
			if !reflect.DeepEqual(got[0], tc.want) {
				t.Fatalf("ledger = %+v, muốn %+v", got[0], tc.want)
			}
		})
	}
}

func TestApplyForeshadowRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name         string
		steps        []applyStep
		wantToolArgs bool
	}{
		{
			name: "plant trùng id ở chương khác",
			steps: []applyStep{
				{1, ForeshadowUpdate{ID: "a", Action: "plant", Description: "x"}},
				{2, ForeshadowUpdate{ID: "a", Action: "plant", Description: "y"}},
			},
			wantToolArgs: true,
		},
		{
			name:  "plant thiếu description",
			steps: []applyStep{{1, ForeshadowUpdate{ID: "a", Action: "plant"}}},
		},
		{
			name:         "due_by không vượt chương gieo",
			steps:        []applyStep{{3, ForeshadowUpdate{ID: "a", Action: "plant", Description: "x", DueBy: 3}}},
			wantToolArgs: true,
		},
		{
			// 同章重放的 plant 也必须过 due_by 校验：先在第 2 章合法入账，
			// 再以 due_by=2 重复 plant——此前校验只挡新条目，重复路径会
			// 静默写入已过期的 due_by。
			name: "plant lặp cùng chương không được chấp nhận due_by quá hạn",
			steps: []applyStep{
				{2, ForeshadowUpdate{ID: "a", Action: "plant", Description: "x"}},
				{2, ForeshadowUpdate{ID: "a", Action: "plant", Description: "x", DueBy: 2}},
			},
			wantToolArgs: true,
		},
		{
			name:  "advance id chưa biết",
			steps: []applyStep{{1, ForeshadowUpdate{ID: "missing", Action: "advance"}}},
		},
		{
			name:  "resolve id chưa biết",
			steps: []applyStep{{1, ForeshadowUpdate{ID: "missing", Action: "resolve"}}},
		},
		{
			name:  "abandon id chưa biết",
			steps: []applyStep{{1, ForeshadowUpdate{ID: "missing", Action: "abandon", Reason: "bỏ"}}},
		},
		{
			name:  "action không hợp lệ",
			steps: []applyStep{{1, ForeshadowUpdate{ID: "a", Action: "drop"}}},
		},
		{
			name:  "id rỗng",
			steps: []applyStep{{1, ForeshadowUpdate{Action: "advance"}}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := applyAll(nil, map[string]int{}, tc.steps)
			if err == nil {
				t.Fatal("muốn lỗi, nhận nil")
			}
			if tc.wantToolArgs && !errors.Is(err, errs.ErrToolArgs) {
				t.Fatalf("muốn ErrToolArgs, nhận %v", err)
			}
		})
	}
}

func TestApplyForeshadowAbandonRequiresReason(t *testing.T) {
	entries, idx := seedLedger(t)
	_, err := ApplyForeshadow(entries, idx, 2, ForeshadowUpdate{ID: "a", Action: "abandon"})
	if !errors.Is(err, errs.ErrToolArgs) {
		t.Fatalf("abandon thiếu reason phải là ErrToolArgs, nhận %v", err)
	}
	if !strings.Contains(err.Error(), "reason") {
		t.Fatalf("thông điệp phải nói thiếu reason: %v", err)
	}
}

func seedLedger(t *testing.T) ([]ForeshadowEntry, map[string]int) {
	t.Helper()
	entries, err := ApplyForeshadow(nil, map[string]int{}, 1,
		ForeshadowUpdate{ID: "a", Action: "plant", Description: "mô tả"})
	if err != nil {
		t.Fatal(err)
	}
	return entries, map[string]int{"a": 0}
}

func TestApplyForeshadowEvidenceIsDeduplicated(t *testing.T) {
	entries, idx := seedLedger(t)
	update := ForeshadowUpdate{ID: "a", Action: "advance", Quote: "trích nguyên văn"}
	var err error
	for range 2 {
		entries, err = ApplyForeshadow(entries, idx, 2, update)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(entries[0].Evidence) != 1 {
		t.Fatalf("evidence phải được khử trùng: %+v", entries[0].Evidence)
	}
	if entries[0].Evidence[0] != (EvidenceRef{Chapter: 2, Action: "advance", Quote: "trích nguyên văn"}) {
		t.Fatalf("evidence sai: %+v", entries[0].Evidence[0])
	}
}

func TestThreadHealth(t *testing.T) {
	tests := []struct {
		name               string
		entry              ForeshadowEntry
		chapter, completed int
		overdue, dormant   bool
	}{
		{
			name:    "quá hạn khi đã qua due_by",
			entry:   ForeshadowEntry{Status: "planted", PlantedAt: 1, DueBy: 3},
			chapter: 5, completed: 4, overdue: true,
		},
		{
			name:    "đúng chương due_by chưa tính quá hạn",
			entry:   ForeshadowEntry{Status: "planted", PlantedAt: 1, DueBy: 5},
			chapter: 5, completed: 4,
		},
		{
			name:    "đã thu thì không quá hạn",
			entry:   ForeshadowEntry{Status: "resolved", PlantedAt: 1, DueBy: 3, ResolvedAt: 4},
			chapter: 5, completed: 4,
		},
		{
			name:    "đã bỏ tuyến thì không quá hạn",
			entry:   ForeshadowEntry{Status: "abandoned", PlantedAt: 1, DueBy: 3},
			chapter: 5, completed: 4,
		},
		{
			name:    "vừa được đẩy thì không ngủ quên",
			entry:   ForeshadowEntry{Status: "advanced", PlantedAt: 1, LastTouched: 20},
			chapter: 21, completed: 20,
		},
		{
			name:    "last_touched cũ thì ngủ quên dù planted_at xa hơn",
			entry:   ForeshadowEntry{Status: "advanced", PlantedAt: 1, LastTouched: 2},
			chapter: 20, completed: 19, dormant: true,
		},
		{
			name:    "sổ cái cũ không có last_touched lùi về planted_at",
			entry:   ForeshadowEntry{Status: "planted", PlantedAt: 1},
			chapter: 20, completed: 19, dormant: true,
		},
		{
			name:    "ngưỡng theo số chương đã hoàn thành",
			entry:   ForeshadowEntry{Status: "planted", PlantedAt: 1},
			chapter: 8, completed: 60,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			overdue, dormant := ThreadHealth(tc.entry, tc.chapter, tc.completed)
			if overdue != tc.overdue || dormant != tc.dormant {
				t.Fatalf("ThreadHealth = (%v, %v), muốn (%v, %v)", overdue, dormant, tc.overdue, tc.dormant)
			}
		})
	}
}

func TestForeshadowDormantThreshold(t *testing.T) {
	for _, tc := range []struct{ completed, want int }{
		{0, 8}, {20, 8}, {23, 8}, {24, 8}, {27, 9}, {60, 20},
	} {
		if got := ForeshadowDormantThreshold(tc.completed); got != tc.want {
			t.Fatalf("ForeshadowDormantThreshold(%d) = %d, muốn %d", tc.completed, got, tc.want)
		}
	}
}
