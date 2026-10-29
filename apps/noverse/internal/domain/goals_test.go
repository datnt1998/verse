package domain

import (
	"slices"
	"testing"
)

// TestGoalsFromContract pin ID 的确定性编号：rb/fm/cc 各自从 1 开始连续，
// 空白条目跳过但不占位；非二值项不进入清单。
func TestGoalsFromContract(t *testing.T) {
	goals := GoalsFromContract(7, ChapterContract{
		RequiredBeats:    []string{"  主角通过第一关 ", "", "埋下试炼邀请", "   "},
		ForbiddenMoves:   []string{"不得揭露师尊身份"},
		ContinuityChecks: []string{"左臂旧伤未愈", " "},
		EvaluationFocus:  []string{"节奏"},
		EmotionTarget:    "紧张",
		PayoffPoints:     []string{"回应伏笔"},
		HookGoal:         "追读",
	})
	want := []ChapterGoal{
		{Chapter: 7, ID: "rb1", Kind: "rb", Text: "主角通过第一关"},
		{Chapter: 7, ID: "rb2", Kind: "rb", Text: "埋下试炼邀请"},
		{Chapter: 7, ID: "fm1", Kind: "fm", Text: "不得揭露师尊身份"},
		{Chapter: 7, ID: "cc1", Kind: "cc", Text: "左臂旧伤未愈"},
	}
	if !slices.Equal(goals, want) {
		t.Fatalf("goals = %+v, want %+v", goals, want)
	}

	if got := GoalsFromContract(1, ChapterContract{}); len(got) != 0 {
		t.Fatalf("empty contract = %+v, want no goals", got)
	}
}

func check(id, status string) GoalCheck {
	return GoalCheck{ID: id, Status: status}
}

// TestDeriveContractStatus pin 汇总规则：全 pass → met；rb+fm 的 fail 过半 →
// missed；其余有 fail/unknown → partial；misses 按 "[id|status] text" 成串。
func TestDeriveContractStatus(t *testing.T) {
	goals := GoalsFromContract(1, ChapterContract{
		RequiredBeats:    []string{"A", "B"},
		ForbiddenMoves:   []string{"C"},
		ContinuityChecks: []string{"D"},
	})

	for _, tc := range []struct {
		name       string
		checks     []GoalCheck
		wantStatus string
		wantMisses []string
	}{
		{
			name: "all pass",
			checks: []GoalCheck{
				check("rb1", "pass"), check("rb2", "pass"), check("fm1", "pass"), check("cc1", "pass"),
			},
			wantStatus: "met",
		},
		{
			name: "one unknown is partial not met",
			checks: []GoalCheck{
				check("rb1", "pass"), check("rb2", "pass"), check("fm1", "pass"), check("cc1", "unknown"),
			},
			wantStatus: "partial",
			wantMisses: []string{"[cc1|unknown] D"},
		},
		{
			name: "single rb fail below half stays partial",
			checks: []GoalCheck{
				check("rb1", "fail"), check("rb2", "pass"), check("fm1", "pass"), check("cc1", "pass"),
			},
			wantStatus: "partial",
			wantMisses: []string{"[rb1|fail] A"},
		},
		{
			name: "half of rb+fm failed is missed",
			checks: []GoalCheck{
				check("rb1", "fail"), check("rb2", "pass"), check("fm1", "fail"), check("cc1", "pass"),
			},
			wantStatus: "missed",
			wantMisses: []string{"[rb1|fail] A", "[fm1|fail] C"},
		},
		{
			name: "cc fail does not count toward binary half",
			checks: []GoalCheck{
				check("rb1", "pass"), check("rb2", "pass"), check("fm1", "pass"), check("cc1", "fail"),
			},
			wantStatus: "partial",
			wantMisses: []string{"[cc1|fail] D"},
		},
		{
			name: "fm pass rb all fail is missed",
			checks: []GoalCheck{
				check("rb1", "fail"), check("rb2", "fail"), check("fm1", "pass"), check("cc1", "pass"),
			},
			wantStatus: "missed",
			wantMisses: []string{"[rb1|fail] A", "[rb2|fail] B"},
		},
		{
			name: "missing check counts as unknown",
			checks: []GoalCheck{
				check("rb1", "pass"), check("rb2", "pass"), check("fm1", "pass"),
			},
			wantStatus: "partial",
			wantMisses: []string{"[cc1|unknown] D"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, misses := DeriveContractStatus(goals, tc.checks)
			if status != tc.wantStatus {
				t.Fatalf("status = %q, want %q", status, tc.wantStatus)
			}
			if !slices.Equal(misses, tc.wantMisses) {
				t.Fatalf("misses = %v, want %v", misses, tc.wantMisses)
			}
		})
	}
}

// TestDeriveContractStatusEmpty pin 无清单不产出状态——调用方据此决定是否
// 覆盖模型提交的值。
func TestDeriveContractStatusEmpty(t *testing.T) {
	if status, misses := DeriveContractStatus(nil, nil); status != "" || misses != nil {
		t.Fatalf("empty = %q %v, want empty", status, misses)
	}
	if status, misses := DeriveContractStatus(
		GoalsFromContract(1, ChapterContract{RequiredBeats: []string{"A"}}), nil,
	); status != "" || misses != nil {
		t.Fatalf("no checks = %q %v, want empty", status, misses)
	}
}
