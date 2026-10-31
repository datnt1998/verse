package editor

import (
	"errors"
	"strings"
	"testing"
)

func TestCloneBeginPolicyUpstreamDecisions(t *testing.T) {
	session := Object{"jobId": "job", "projectRevision": "sha256:rev-a", "settingsHash": "settings", "artifactHash": "artifacts"}
	cases := []struct {
		name  string
		input CloneBeginPolicy
		gate  string
	}{
		{"missing score", CloneBeginPolicy{Session: session}, "missing_score"},
		{"stale revision", CloneBeginPolicy{Session: session, Score: Object{"total": 100}, StaleReason: "project revision changed"}, "stale_score"},
		{"below threshold and hard blockers", CloneBeginPolicy{Session: session, Score: Object{"total": 80}, Fresh: true, Reasons: []string{"captions do not cover speech"}}, "below_threshold"},
		{"fresh passing", CloneBeginPolicy{Session: session, Score: Object{"total": 100}, Fresh: true}, ""},
		{"accepted missing score", CloneBeginPolicy{Session: session, Accepted: true}, ""},
		{"accepted stale score", CloneBeginPolicy{Session: session, Score: Object{"total": 100}, Accepted: true}, ""},
		{"accepted failing checklist", CloneBeginPolicy{Session: session, Score: Object{"total": 80}, Fresh: true, Accepted: true, Reasons: []string{"captions"}}, ""},
		{"replaced template media", CloneBeginPolicy{Session: session, Score: Object{"total": 100}, Fresh: true, TemplateDrift: []string{"media: đã thay"}, Recovery: "Khôi phục file gốc"}, "template_media_drift"},
		{"deleted template media", CloneBeginPolicy{Session: session, Score: Object{"total": 100}, Fresh: true, TemplateDrift: []string{"media: đã xoá"}, Recovery: "Thêm lại file"}, "template_media_drift"},
		{"acceptance never waives template integrity", CloneBeginPolicy{Session: session, Accepted: true, TemplateDrift: []string{"media: đã thay"}, Recovery: "Khôi phục file gốc"}, "template_media_drift"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bound, e := EvaluateCloneBeginPolicy(tc.input)
			if bound["projectRevision"] != "sha256:rev-a" {
				t.Fatalf("session = %v", bound)
			}
			if tc.gate == "" {
				if e != nil {
					t.Fatal(e)
				}
			} else {
				var policyError *Error
				if !errors.As(e, &policyError) || policyError.Extra["cloneGate"] != tc.gate {
					t.Fatalf("error = %v", e)
				}
				if tc.input.Recovery != "" && !strings.Contains(e.Error(), tc.input.Recovery) {
					t.Fatalf("missing recovery: %v", e)
				}
			}
			bound["projectRevision"] = "mutated"
			if session["projectRevision"] != "sha256:rev-a" {
				t.Fatal("session aliases caller")
			}
		})
	}
}

func TestCloneCommitPolicyUpstreamDecisions(t *testing.T) {
	passing := Object{"total": 100, "passed": true}
	failed := Object{"total": 80, "passed": false}
	probed := Object{"facts": Object{"width": 1920, "height": 1080}, "lufs": -14.1}
	cases := []struct {
		name   string
		input  CloneCommitPolicy
		ready  bool
		reason string
		score  bool
	}{
		{"clean staged MP4", CloneCommitPolicy{JobPresent: true, Probed: probed, Score: passing}, true, "", true},
		{"revision drift skips scoring", CloneCommitPolicy{JobPresent: true, Probed: probed, Score: passing, DriftReasons: []string{"Project đã đổi sau khi bắt đầu xuất — xuất lại từ đầu"}}, false, "Project đã đổi", false},
		{"probe failure", CloneCommitPolicy{JobPresent: true, ProbeError: "ffprobe missing", Score: passing}, false, "ffprobe missing", false},
		{"orphan job", CloneCommitPolicy{Probed: probed, Score: passing}, false, "không còn thuộc job", false},
		{"low-resolution actual output", CloneCommitPolicy{JobPresent: true, Probed: probed, Score: failed, ScoreReasons: []string{"Cần 1920×1080"}}, false, "1920×1080", true},
		{"muted source after staging", CloneCommitPolicy{JobPresent: true, Probed: probed, Score: failed, ScoreReasons: []string{"Âm thanh gốc bị tắt"}}, false, "Âm thanh gốc", true},
		{"accepted checklist remains diagnostic", CloneCommitPolicy{JobPresent: true, Probed: probed, Score: failed, Accepted: true, ScoreReasons: []string{"Âm thanh gốc bị tắt"}}, true, "", true},
		{"accepted broken file refuses", CloneCommitPolicy{JobPresent: true, Accepted: true, ProbeError: "moov atom not found", Score: failed}, false, "moov atom", false},
		{"replaced template media", CloneCommitPolicy{JobPresent: true, Probed: probed, Score: passing, TemplateReasons: []string{"Template đã thay. Khôi phục file gốc"}}, false, "Khôi phục file gốc", false},
		{"deleted template media", CloneCommitPolicy{JobPresent: true, Probed: probed, Score: passing, TemplateReasons: []string{"Template đã xoá. Thêm lại file"}}, false, "Thêm lại file", false},
		{"tampered template snapshot", CloneCommitPolicy{JobPresent: true, Probed: probed, Score: passing, TemplateError: "Bản lưu template đã bị sửa"}, false, "đã bị sửa", false},
		{"unbound template", CloneCommitPolicy{JobPresent: true, Probed: probed, Score: failed, ScoreReasons: []string{"Thiếu bản lưu template"}}, false, "bản lưu template", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics, e := EvaluateCloneCommitPolicy(tc.input)
			if (e == nil) != tc.ready {
				t.Fatalf("ready = %v, error = %v", tc.ready, e)
			}
			if (exportObject(diagnostics["score"]) != nil) != tc.score {
				t.Fatalf("score = %v", diagnostics["score"])
			}
			if tc.reason != "" {
				if e == nil || !strings.Contains(e.Error(), tc.reason) {
					t.Fatalf("error = %v", e)
				}
				var policyError *Error
				if !errors.As(e, &policyError) || policyError.Extra["cloneGate"] != "final_check_failed" {
					t.Fatalf("error = %v", e)
				}
			}
		})
	}
}
