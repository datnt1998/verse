package queue

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/datnt1998/verse/core/llm"
	"github.com/datnt1998/verse/core/llmretry"
)

// scoutGen is a scripted llmretry.Generator: each call returns the next canned
// text (or a plain non-retryable error). It counts calls so tests can prove at
// most one interpretation happens per cycle.
type scoutGen struct {
	texts []string
	err   error
	calls int
}

func (g *scoutGen) Generate(_ context.Context, _ []llm.Message, _ []llm.ToolSpec, _ ...llm.CallOption) (*llm.LLMResponse, error) {
	g.calls++
	if g.err != nil {
		return nil, g.err
	}
	if len(g.texts) == 0 {
		return nil, errors.New("unexpected model call")
	}
	text := g.texts[0]
	g.texts = g.texts[1:]
	return &llm.LLMResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{llm.TextBlock(text)}}}, nil
}

// scoutStore only implements the reads the policy path touches; every other
// Store method resolves through the embedded nil interface and panics if the
// policy path ever calls it.
type scoutStore struct {
	Store
	info map[string]struct {
		Title       string
		MedianViews *int64
	}
	videos map[string]RivalVideoView
}

func (f *scoutStore) ChannelInfoOf(_ context.Context, ids []string) (map[string]struct {
	Title       string
	MedianViews *int64
}, error) {
	out := map[string]struct {
		Title       string
		MedianViews *int64
	}{}
	for _, id := range ids {
		if v, ok := f.info[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

func (f *scoutStore) ListVideosByIDs(_ context.Context, ids []string) (map[string]RivalVideoView, error) {
	out := map[string]RivalVideoView{}
	for _, id := range ids {
		if v, ok := f.videos[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

func scoutService(gen *scoutGen) *Service {
	return New(Options{
		Store:      &scoutStore{videos: map[string]RivalVideoView{}},
		ScoutModel: func(context.Context) (llmretry.Generator, error) { return gen, nil },
	})
}

func linksOf(ids ...string) []ChannelSourceRow {
	out := make([]ChannelSourceRow, len(ids))
	for i, id := range ids {
		out[i] = ChannelSourceRow{RivalID: id, LinkedAt: time.Now()}
	}
	return out
}

func TestScoutPolicyRestrictsToLinkedRival(t *testing.T) {
	gen := &scoutGen{texts: []string{`{"onlyRivalId":"B","minDurationSec":null,"unsupported":[]}`}}
	svc := scoutService(gen)
	candidates := map[string][]string{"A": {"a1", "a2"}, "B": {"b1"}}
	policy, err := svc.resolveScoutPolicy(context.Background(), "chỉ lấy kênh B", linksOf("A", "B"), candidates)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if policy.OnlyRivalID == nil || *policy.OnlyRivalID != "B" {
		t.Fatalf("policy = %+v, muốn onlyRivalId B", policy)
	}
	got, err := svc.applyScoutPolicy(context.Background(), policy, linksOf("A", "B"), candidates)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(got) != 1 || len(got["B"]) != 1 || got["B"][0] != "b1" {
		t.Fatalf("candidates = %v, muốn chỉ còn b1 của B", got)
	}
	if gen.calls != 1 {
		t.Fatalf("model calls = %d, muốn đúng 1", gen.calls)
	}
}

func TestScoutPolicyMinDurationKeepsLongVideos(t *testing.T) {
	store := &scoutStore{videos: map[string]RivalVideoView{
		"short":  {ID: "short", DurationSec: new(int32(300))},
		"long":   {ID: "long", DurationSec: new(int32(2000))},
		"unread": {ID: "unread"},
	}}
	gen := &scoutGen{texts: []string{`{"onlyRivalId":null,"minDurationSec":1200,"unsupported":[]}`}}
	svc := New(Options{Store: store, ScoutModel: func(context.Context) (llmretry.Generator, error) { return gen, nil }})
	candidates := map[string][]string{"A": {"short", "long", "unread", "missing"}}
	policy, err := svc.resolveScoutPolicy(context.Background(), "chỉ lấy video trên 20 phút", linksOf("A"), candidates)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	got, err := svc.applyScoutPolicy(context.Background(), policy, linksOf("A"), candidates)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(got["A"]) != 1 || got["A"][0] != "long" {
		t.Fatalf("candidates = %v, muốn chỉ còn long", got)
	}
}

func TestScoutPolicyCannotLoosenConfiguredFloor(t *testing.T) {
	store := &scoutStore{videos: map[string]RivalVideoView{
		"v1": {ID: "v1", DurationSec: new(int32(400))}, // qua floor 300 nhưng dưới chỉ dẫn 500
	}}
	svc := New(Options{Store: store})
	// SQL đã lọc theo floor 300 nên chỉ v1 còn; chỉ dẫn 500 phải lọc tiếp.
	links := linksOf("A")
	links[0].MinDurationSec = new(300)
	got, err := svc.applyScoutPolicy(context.Background(), scoutPolicy{MinDurationSec: new(500)}, links, map[string][]string{"A": {"v1"}})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(got["A"]) != 0 {
		t.Fatalf("video dưới ngưỡng chỉ dẫn vẫn được giữ: %v", got)
	}
	// Floor cao hơn chỉ dẫn: không cần đọc duration, danh sách giữ nguyên.
	links[0].MinDurationSec = new(3600)
	candidates := map[string][]string{"A": {"v1"}}
	got, err = svc.applyScoutPolicy(context.Background(), scoutPolicy{MinDurationSec: new(500)}, links, candidates)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(got["A"]) != 1 {
		t.Fatalf("floor cao hơn chỉ dẫn nhưng video bị mất: %v", got)
	}
}

func TestScoutPolicyRejectsUnlinkedRival(t *testing.T) {
	bad := `{"onlyRivalId":"stranger","minDurationSec":null,"unsupported":[]}`
	gen := &scoutGen{texts: []string{bad, bad, bad, bad}}
	svc := scoutService(gen)
	candidates := map[string][]string{"A": {"a1"}}
	_, err := svc.resolveScoutPolicy(context.Background(), "chỉ lấy kênh chưa liên kết", linksOf("A"), candidates)
	if err == nil {
		t.Fatalf("err = %v, muốn lỗi đối thủ chưa liên kết", err)
	}
}

func TestScoutPolicyRejectsUnsupportedAndAmbiguous(t *testing.T) {
	for i, text := range []string{
		`{"onlyRivalId":null,"minDurationSec":null,"unsupported":["chỉ lấy video trên 10k view"]}`,
		`{"onlyRivalId":null,"minDurationSec":null,"unsupported":[]}`,
	} {
		gen := &scoutGen{texts: []string{text}}
		svc := scoutService(gen)
		_, err := svc.resolveScoutPolicy(context.Background(), "chỉ dẫn khó", linksOf("A"), map[string][]string{"A": {"a1"}})
		if err == nil {
			t.Fatalf("case %d: chỉ dẫn %s bị chấp nhận", i, text)
		}
	}
}

func TestScoutPolicySkipsModelWithoutNoteOrCandidates(t *testing.T) {
	for name, tc := range map[string]struct {
		note       string
		candidates map[string][]string
	}{
		"không chỉ dẫn":       {"", map[string][]string{"A": {"a1"}}},
		"chỉ dẫn nhưng trống": {"chỉ lấy kênh A", map[string][]string{"A": {}}},
	} {
		gen := &scoutGen{texts: []string{`{"onlyRivalId":null,"minDurationSec":1,"unsupported":[]}`}}
		svc := scoutService(gen)
		policy, err := svc.resolveScoutPolicy(context.Background(), tc.note, linksOf("A"), tc.candidates)
		if err != nil || policy.restricts() {
			t.Fatalf("%s: policy=%+v err=%v, muốn rỗng", name, policy, err)
		}
		if gen.calls != 0 {
			t.Fatalf("%s: model calls = %d, muốn 0", name, gen.calls)
		}
	}
}

func TestScoutPolicyFailsWithoutModel(t *testing.T) {
	svc := New(Options{Store: &scoutStore{videos: map[string]RivalVideoView{}}})
	_, err := svc.resolveScoutPolicy(context.Background(), "chỉ lấy kênh A", linksOf("A"), map[string][]string{"A": {"a1"}})
	if err == nil {
		t.Fatalf("err = %v, muốn lỗi thiếu model", err)
	}
}

func TestScoutPolicyRejectsModelOutOfBoundsDuration(t *testing.T) {
	for _, sec := range []int{0, -5, scoutMaxMinDurationSec + 1} {
		text := fmt.Sprintf(`{"onlyRivalId":null,"minDurationSec":%d,"unsupported":[]}`, sec)
		gen := &scoutGen{texts: []string{text, text, text, text}}
		svc := scoutService(gen)
		_, err := svc.resolveScoutPolicy(context.Background(), "lọc thời lượng", linksOf("A"), map[string][]string{"A": {"a1"}})
		if err == nil {
			t.Fatalf("sec=%d: chỉ dẫn ngoài giới hạn bị chấp nhận", sec)
		}
	}
}
