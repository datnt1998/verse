package domain

import (
	"strings"
	"testing"
	"unicode"
)

// Mã pha thật của từng bước (pipeline phát ra) — câu hiện cho người đọc không
// được chứa lại mã này.
var clonePhaseSamples = map[CloneStep][]string{
	CloneDownload:   {"metadata", "download", "merge", "probe", "transcode", "import", "publish"},
	CloneBackdrop:   {"refresh", "pick", "fetch"},
	CloneTranscript: {"decode", "vad", "asr", "align", "publish"},
	CloneImages:     {"images"},
	CloneScenes:     {"keyframes"},
	CloneScore:      {"measuring"},
	CloneExport:     {"rendering", "rendering-again", "uploading", "uploading-again", "compressing"},
}

func TestClonePhaseVerbCoversEveryProducedPhase(t *testing.T) {
	for step, phases := range clonePhaseSamples {
		for _, phase := range phases {
			verb := clonePhaseVerb(step, phase)
			if verb == "" {
				t.Fatalf("%s/%s: pha thật nhưng không có câu Việt", step, phase)
			}
			if strings.Contains(strings.ToLower(verb), phase) {
				t.Fatalf("%s/%s: câu còn chứa mã pha: %q", step, phase, verb)
			}
			if !hasVietnameseLetter(verb) {
				t.Fatalf("%s/%s: câu không phải tiếng Việt: %q", step, phase, verb)
			}
			if strings.HasPrefix(strings.ToLower(verb), "đang ") {
				t.Fatalf("%s/%s: câu đã có \"đang\", ghép sau tên trợ lý sẽ lặp: %q", step, phase, verb)
			}
		}
	}
}

func TestClonePhaseVerbKeepsStepMeaning(t *testing.T) {
	download := clonePhaseVerb(CloneDownload, "publish")
	transcript := clonePhaseVerb(CloneTranscript, "publish")
	if download == transcript {
		t.Fatalf("publish phải khác nghĩa theo bước, cùng trả %q", download)
	}
	if clonePhaseVerb(CloneExport, "rendering") == clonePhaseVerb(CloneExport, "rendering-again") {
		t.Fatal("dựng lại phải khác câu dựng lần đầu")
	}
}

func TestClonePhaseVerbNeverLeaksUnknownCode(t *testing.T) {
	for _, phase := range []string{"brand_new_token", "some_future_phase"} {
		if verb := clonePhaseVerb(CloneTranscript, phase); verb != "" {
			t.Fatalf("mã lạ %q phải trả rỗng để chỗ gọi dùng câu chung, nhận %q", phase, verb)
		}
	}
}

func TestClonePhaseVerbRejectsUnmappedPhrases(t *testing.T) {
	// Máy chủ phát mã cố định; câu lạ (kể cả có dấu) không được tự lọt vào UI.
	for _, phase := range []string{"Đang đọc thông tin nguồn", "Café rendering déjà vu", "Phase mới"} {
		if verb := clonePhaseVerb(CloneDownload, phase); verb != "" {
			t.Fatalf("pha lạ %q phải trả rỗng để dùng câu chung, nhận %q", phase, verb)
		}
	}
}

func TestRunningSentenceHasNoEnglishPhase(t *testing.T) {
	for step, phases := range clonePhaseSamples {
		agent := CloneAgentOf(step)
		if agent == "" {
			continue
		}
		for _, phase := range phases {
			row := CloneStepRecord{Step: step, Status: CloneRunning, Progress: CloneProgress{Phase: phase}}
			sentence := runningSentence(agent, row)
			if sentence == "" {
				t.Fatalf("%s/%s: câu đang làm rỗng", step, phase)
			}
			if strings.Contains(strings.ToLower(sentence), phase) {
				t.Fatalf("%s/%s: câu đang làm còn mã tiếng Anh: %q", step, phase, sentence)
			}
		}
	}
}

func TestRunningSentenceUnknownPhaseStaysVietnamese(t *testing.T) {
	row := CloneStepRecord{Step: CloneTranscript, Status: CloneRunning, Progress: CloneProgress{Phase: "future_code"}}
	sentence := runningSentence(AgentScout, row)
	if strings.Contains(sentence, "future_code") {
		t.Fatalf("mã lạ lọt vào câu: %q", sentence)
	}
	if !strings.HasPrefix(sentence, "Trinh sát đang") {
		t.Fatalf("mất tên trợ lý: %q", sentence)
	}
}

func hasVietnameseLetter(text string) bool {
	for _, r := range text {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}
