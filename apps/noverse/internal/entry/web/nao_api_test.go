package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// naoFixture dựng dữ liệu Não truyện tối thiểu trong truyện cũ "_/oldbook":
// 3 chương đã viết trên 8 chương đại cương, một cặp nhân vật có xưng hô đổi ở ch.3
// và quan hệ đổi ở ch.2.
func naoFixture(t *testing.T, tg *testGateway) string {
	t.Helper()
	engine := filepath.Join(tg.root, "oldbook", "output", "novel")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(engine, rel)
		must(os.MkdirAll(filepath.Dir(path), 0o755))
		must(os.WriteFile(path, []byte(body), 0o644))
	}
	write("characters.json", `[
  {"id":"nv:tri-ha","name":"Tri Hạ","role":"protagonist","description":"Người kể."},
  {"id":"nv:nhUOC-vy","name":"Nhược Vy","role":"antagonist","description":"Phản diện."},
  {"id":"nv:ky-duong","name":"Kỳ Dương","role":"supporting","description":"Bạn diễn."}
]`)
	write("address_terms.json", `[
  {"from":"nv:tri-ha","to":"nv:nhUOC-vy","calls":"em","self":"anh","from_chapter":1,"reason":"ch.1","scope":"truyen","source":"chương 1"},
  {"from":"nv:nhUOC-vy","to":"nv:ky-duong","calls":"anh","self":"em","from_chapter":1,"until_chapter":2,"reason":"chỉ ở ch.1","scope":"truyen","source":"chương 1"}
]`)
	write("relationship_log.jsonl", `{"chapter":1,"from":"nv:tri-ha","to":"nv:nhUOC-vy","relation":"Bạn diễn cũ","scope":"truyen","source":"chương 1"}
{"chapter":2,"from":"nv:tri-ha","to":"nv:nhUOC-vy","relation":"Đối đầu","type":"enemy","scope":"truyen","source":"chương 2"}
{"chapter":2,"from":"nv:tri-ha","to":"nv:nhUOC-vy","relation":"Đối đầu","type":"enemy","scope":"truyen","source":"chương 2"}
{"chapter":2,"from":"nv:tri-ha","to":"nv:ky-duong","relation":"Đồng minh","type":"ally","scope":"truyen","source":"chương 2"}
`)
	write("meta/progress.json", `{"phase":"writing","current_chapter":4,"total_chapters":8,"completed_chapters":[1,2,3],"flow":"writing"}`)
	write("outline.json", `[
  {"chapter":1,"title":"Mở màn"},{"chapter":2,"title":"Đối đầu"},{"chapter":3,"title":"Buổi đọc"},
  {"chapter":4,"title":"Vỡ"},{"chapter":5,"title":"Sau"},{"chapter":6,"title":"Tối"},{"chapter":7,"title":"Đêm"},{"chapter":8,"title":"Kết"}
]`)
	write("summaries/01.json", `{"chapter":1,"title":"Mở màn","summary":"","characters":["Tri Hạ","Nhược Vy"],"key_events":[]}`)
	write("summaries/02.json", `{"chapter":2,"title":"Đối đầu","summary":"","characters":["Tri Hạ","Nhược Vy","Kỳ Dương"],"key_events":[]}`)
	write("summaries/03.json", `{"chapter":3,"title":"Buổi đọc","summary":"","characters":["Tri Hạ","Nhược Vy"],"key_events":[]}`)
	write("foreshadow_ledger.json", `[{"id":"khan-do","description":"Khăn đỏ trong túi","planted_at":2,"status":"planted"}]`)
	write("timeline.json", `[{"chapter":2,"time":"đêm","event":"Buổi đọc thoại","characters":["Tri Hạ"]}]`)
	write("world_rules.json", `[{"category":"magic","rule":"Không ai đọc được ý nghĩ","boundary":"Chỉ người kể"}]`)
	return engine
}

type naoTermsBody struct {
	Terms []struct {
		ID           string `json:"id"`
		From         string `json:"from"`
		FromName     string `json:"fromName"`
		To           string `json:"to"`
		ToName       string `json:"toName"`
		Call         string `json:"call"`
		Self         string `json:"self"`
		FromChapter  int    `json:"fromChapter"`
		UntilChapter int    `json:"untilChapter"`
		Reason       string `json:"reason"`
	} `json:"terms"`
	Relations []struct {
		Chapter int    `json:"chapter"`
		AName   string `json:"aName"`
		BName   string `json:"bName"`
		Before  string `json:"before"`
		After   string `json:"after"`
		Reason  string `json:"reason"`
		Type    string `json:"type"`
	} `json:"relations"`
	LatestWritten int `json:"latestWritten"`
	Writing       *struct {
		Chapter int    `json:"chapter"`
		Role    string `json:"role"`
		State   string `json:"state"`
	} `json:"writing"`
}

func naoTerms(t *testing.T, w interface {
	Result() *http.Response
}) naoTermsBody {
	t.Helper()
	body, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	var out naoTermsBody
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	return out
}

func naoTermByCall(t *testing.T, body naoTermsBody, call string) int {
	t.Helper()
	for i, term := range body.Terms {
		if term.Call == call {
			return i
		}
	}
	t.Fatalf("term with call %q not found: %+v", call, body.Terms)
	return -1
}

// naoFakeWriting giả lập worker đang viết lại chương 4 bằng vai Người viết.
func naoFakeWriting(t *testing.T, tg *testGateway) {
	t.Helper()
	naoFakeWritingFlow(t, tg, "rewriting")
}

// naoFakeWritingFlow giả lập worker đang chạy flow cho trước ở chương 4.
func naoFakeWritingFlow(t *testing.T, tg *testGateway, flow string) {
	t.Helper()
	done := make(chan struct{})
	var once sync.Once
	slot := tg.g.slot("_/oldbook")
	slot.worker = fakeWorker(done, &once)
	tg.g.setTransport(transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/snapshot" {
			t.Errorf("unexpected worker path %q", r.URL.Path)
		}
		body := `{"isRunning":true,"currentChapter":4,"inProgressChapter":4,"flow":"` + flow + `",` +
			`"agents":[{"name":"architect_long","state":"idle"},{"name":"writer","state":"working"}]}`
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	}))
}

func TestNaoAddressTermsRead(t *testing.T) {
	tg := newTestGateway(t)
	naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	w := tg.do("GET", "/api/projects/_/stories/oldbook/address-terms", admin, "")
	if w.Code != 200 {
		t.Fatalf("get: %d %s", w.Code, w.Body.String())
	}
	body := naoTerms(t, w)
	if body.LatestWritten != 3 {
		t.Fatalf("latestWritten = %d, want 3", body.LatestWritten)
	}
	if body.Writing != nil {
		t.Fatalf("writing = %+v, want null", body.Writing)
	}
	if len(body.Terms) != 2 {
		t.Fatalf("terms = %+v", body.Terms)
	}
	active := body.Terms[naoTermByCall(t, body, "em")]
	if active.FromName != "Tri Hạ" || active.ToName != "Nhược Vy" || active.UntilChapter != 0 || active.FromChapter != 1 {
		t.Fatalf("active term row = %+v", active)
	}
	expired := body.Terms[naoTermByCall(t, body, "anh")]
	if expired.FromChapter != 1 || expired.UntilChapter != 2 || expired.Reason != "chỉ ở ch.1" || expired.ToName != "Kỳ Dương" {
		t.Fatalf("expired term row = %+v", expired)
	}
	if len(body.Relations) != 3 {
		t.Fatalf("relations = %+v", body.Relations)
	}
	if body.Relations[0].Before != "" || body.Relations[0].After != "Bạn diễn cũ" {
		t.Fatalf("first relation = %+v", body.Relations[0])
	}
	if body.Relations[1].Before != "Bạn diễn cũ" || body.Relations[1].After != "Đối đầu" {
		t.Fatalf("second relation = %+v", body.Relations[1])
	}
	// Câu 20: chip quan hệ tô theo type; dữ kiện cũ thiếu type đọc là "rel".
	if body.Relations[0].Type != "rel" || body.Relations[1].Type != "enemy" {
		t.Fatalf("relation types = %q, %q", body.Relations[0].Type, body.Relations[1].Type)
	}
}

func TestNaoWriteLockedWhileEngineWrites(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	naoFakeWriting(t, tg)
	admin := tg.login("admin", "admin-pass")

	w := tg.do("PUT", "/api/projects/_/stories/oldbook/address-terms", admin,
		`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":4,"reason":"đổi"},"apply":"mark"}`)
	if w.Code != http.StatusLocked {
		t.Fatalf("put while writing: %d %s", w.Code, w.Body.String())
	}
	var locked struct {
		Error   string `json:"error"`
		Writing struct {
			Chapter int    `json:"chapter"`
			Role    string `json:"role"`
		} `json:"writing"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &locked); err != nil {
		t.Fatal(err)
	}
	if locked.Writing.Chapter != 4 || locked.Writing.Role != "writer" {
		t.Fatalf("writing = %+v", locked.Writing)
	}
	if !strings.Contains(locked.Error, "chương 4") || !strings.Contains(locked.Error, "khoá") {
		t.Fatalf("error = %q", locked.Error)
	}
	if !strings.Contains(locked.Error, "viết lại") {
		t.Fatalf("flow rewriting phải đọc là viết lại: %q", locked.Error)
	}
	raw, err := os.ReadFile(filepath.Join(engine, "address_terms.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "chị") {
		t.Fatalf("locked write reached disk: %s", raw)
	}

	// Quan hệ cũng bị khoá.
	if code := tg.do("POST", "/api/projects/_/stories/oldbook/relations", admin,
		`{"a":"Tri Hạ","b":"Nhược Vy","after":"Đồng minh","fromChapter":4}`).Code; code != http.StatusLocked {
		t.Fatalf("relation while writing: %d", code)
	}
	// Đọc vẫn được khi engine đang viết, kèm trạng thái khoá để UI vẽ banner.
	w = tg.do("GET", "/api/projects/_/stories/oldbook/address-terms", admin, "")
	if w.Code != 200 {
		t.Fatalf("read while writing: %d", w.Code)
	}
	if got := naoTerms(t, w); got.Writing == nil || got.Writing.Chapter != 4 || got.Writing.Role != "writer" || got.Writing.State != "rewriting" {
		t.Fatalf("get writing = %+v", got.Writing)
	}
}

// TestNaoAddressTermsWritingFirstTime: engine viết chương lần đầu (Câu 23) thì khoá
// vẫn báo đúng chương, nhưng là "writing" chứ không phải "rewriting", và câu khoá
// đọc là "đang viết chương 4".
func TestNaoAddressTermsWritingFirstTime(t *testing.T) {
	tg := newTestGateway(t)
	naoFixture(t, tg)
	naoFakeWritingFlow(t, tg, "writing")
	admin := tg.login("admin", "admin-pass")

	w := tg.do("GET", "/api/projects/_/stories/oldbook/address-terms", admin, "")
	if w.Code != 200 {
		t.Fatalf("get: %d %s", w.Code, w.Body.String())
	}
	body := naoTerms(t, w)
	if body.Writing == nil || body.Writing.Chapter != 4 || body.Writing.State != "writing" {
		t.Fatalf("writing = %+v", body.Writing)
	}

	locked := tg.do("PUT", "/api/projects/_/stories/oldbook/address-terms", admin,
		`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":4,"reason":"đổi"},"apply":"mark"}`)
	if locked.Code != http.StatusLocked {
		t.Fatalf("put while writing first time: %d %s", locked.Code, locked.Body.String())
	}
	var errBody struct {
		Error   string `json:"error"`
		Writing struct {
			State string `json:"state"`
		} `json:"writing"`
	}
	if err := json.Unmarshal(locked.Body.Bytes(), &errBody); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errBody.Error, "đang viết chương 4") || strings.Contains(errBody.Error, "viết lại") {
		t.Fatalf("error = %q", errBody.Error)
	}
	if errBody.Writing.State != "writing" {
		t.Fatalf("locked writing state = %q", errBody.Writing.State)
	}
}

func TestNaoAddressTermNeedsApplyWhenTouchingWritten(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	w := tg.do("PUT", "/api/projects/_/stories/oldbook/address-terms", admin,
		`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":2,"reason":"đổi sớm"}}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("no apply: %d %s", w.Code, w.Body.String())
	}
	var conflict struct {
		Error    string `json:"error"`
		Affected []int  `json:"affected"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &conflict); err != nil {
		t.Fatal(err)
	}
	if len(conflict.Affected) != 2 || conflict.Affected[0] != 2 || conflict.Affected[1] != 3 {
		t.Fatalf("affected = %v", conflict.Affected)
	}
	if !strings.Contains(conflict.Error, "2 chương đã viết") {
		t.Fatalf("error = %q", conflict.Error)
	}
	raw, _ := os.ReadFile(filepath.Join(engine, "address_terms.json"))
	if strings.Contains(string(raw), "chị") {
		t.Fatalf("409 must not write: %s", raw)
	}
}

func TestNaoAddressTermNextKeepsWrittenChapters(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	w := tg.do("PUT", "/api/projects/_/stories/oldbook/address-terms", admin,
		`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":2,"reason":"đổi sớm"},"apply":"next"}`)
	if w.Code != 200 {
		t.Fatalf("next: %d %s", w.Code, w.Body.String())
	}
	body := naoTerms(t, w)
	if len(body.Terms) != 3 {
		t.Fatalf("terms = %+v", body.Terms)
	}
	old := body.Terms[naoTermByCall(t, body, "em")]
	if old.UntilChapter != 3 {
		t.Fatalf("old row must stay in force through the last written chapter: %+v", old)
	}
	fresh := body.Terms[naoTermByCall(t, body, "chị")]
	if fresh.FromChapter != 4 || fresh.UntilChapter != 0 || fresh.FromName != "Tri Hạ" {
		t.Fatalf("new row must start at the first unwritten chapter: %+v", fresh)
	}
	// Dữ liệu phải nói đúng như chính văn: ch.2-3 vẫn gọi "em", ch.4 mới gọi "chị".
	if got := naoCallAt(t, engine, 2); got != "em" {
		t.Fatalf("form at ch.2 = %q, want em", got)
	}
	if got := naoCallAt(t, engine, 3); got != "em" {
		t.Fatalf("form at ch.3 = %q, want em", got)
	}
	if got := naoCallAt(t, engine, 4); got != "chị" {
		t.Fatalf("form at ch.4 = %q, want chị", got)
	}
	progress, err := os.ReadFile(filepath.Join(engine, "meta", "progress.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(progress), "pending_rewrites") {
		t.Fatalf("next must not queue rewrites: %s", progress)
	}
}

// naoCallAt trả cách gọi đang có hiệu lực ở chương N theo address_terms.json.
func naoCallAt(t *testing.T, engine string, chapter int) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(engine, "address_terms.json"))
	if err != nil {
		t.Fatal(err)
	}
	var terms []struct {
		From        string `json:"from"`
		To          string `json:"to"`
		Calls       string `json:"calls"`
		FromChapter int    `json:"from_chapter"`
		Until       int    `json:"until_chapter"`
	}
	if err := json.Unmarshal(raw, &terms); err != nil {
		t.Fatal(err)
	}
	for _, term := range terms {
		if term.From != "nv:tri-ha" || term.To != "nv:nhUOC-vy" {
			continue
		}
		if term.FromChapter <= chapter && (term.Until == 0 || term.Until >= chapter) {
			return term.Calls
		}
	}
	return ""
}

func TestNaoAddressTermMarkQueuesRewrite(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	w := tg.do("PUT", "/api/projects/_/stories/oldbook/address-terms", admin,
		`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":2,"reason":"đổi sớm"},"apply":"mark"}`)
	if w.Code != 200 {
		t.Fatalf("mark: %d %s", w.Code, w.Body.String())
	}
	var progress struct {
		PendingRewrites []int  `json:"pending_rewrites"`
		RewriteReason   string `json:"rewrite_reason"`
		Flow            string `json:"flow"`
	}
	raw, err := os.ReadFile(filepath.Join(engine, "meta", "progress.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &progress); err != nil {
		t.Fatal(err)
	}
	if len(progress.PendingRewrites) != 2 || progress.PendingRewrites[0] != 2 || progress.PendingRewrites[1] != 3 {
		t.Fatalf("pending rewrites = %v", progress.PendingRewrites)
	}
	if progress.Flow != "rewriting" {
		t.Fatalf("flow = %q", progress.Flow)
	}
	want := "Chỉ sửa câu có cách gọi Tri Hạ → Nhược Vy: “em” → “chị”."
	if progress.RewriteReason != want {
		t.Fatalf("rewrite reason = %q, want %q", progress.RewriteReason, want)
	}
	// mark giữ đúng chương người dùng chọn: ch.2 dùng cách gọi mới và được đưa vào hàng đợi.
	if got := naoCallAt(t, engine, 2); got != "chị" {
		t.Fatalf("form at ch.2 after mark = %q, want chị", got)
	}
}

func TestNaoAddressTermFutureChapterSavesWithoutApply(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	w := tg.do("PUT", "/api/projects/_/stories/oldbook/address-terms", admin,
		`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":5,"reason":"sau này"}}`)
	if w.Code != 200 {
		t.Fatalf("future chapter: %d %s", w.Code, w.Body.String())
	}
	var terms []struct {
		FromChapter int `json:"from_chapter"`
	}
	raw, err := os.ReadFile(filepath.Join(engine, "address_terms.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &terms); err != nil {
		t.Fatal(err)
	}
	if len(terms) != 3 || terms[2].FromChapter != 5 {
		t.Fatalf("terms on disk = %+v", terms)
	}
}

func TestNaoRelationsAppend(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	w := tg.do("POST", "/api/projects/_/stories/oldbook/relations", admin,
		`{"a":"Tri Hạ","b":"Nhược Vy","after":"Đồng minh","reason":"cứu nhau ở ch.4","fromChapter":4}`)
	if w.Code != 200 {
		t.Fatalf("post relation: %d %s", w.Code, w.Body.String())
	}
	body := naoTerms(t, w)
	last := body.Relations[len(body.Relations)-1]
	if last.After != "Đồng minh" || last.Before != "Đối đầu" || last.Reason != "cứu nhau ở ch.4" || last.Chapter != 4 {
		t.Fatalf("relation row = %+v", last)
	}
	raw, err := os.ReadFile(filepath.Join(engine, "relationship_log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"reason":"cứu nhau ở ch.4"`) {
		t.Fatalf("log line missing reason: %s", raw)
	}
	if !strings.Contains(string(raw), `"source":"người dùng"`) {
		t.Fatalf("log line missing user source: %s", raw)
	}
	if !strings.Contains(string(raw), `"type":"rel"`) {
		t.Fatalf("log line missing default type: %s", raw)
	}
}

func TestNaoRelationTypeStoredAndExposed(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	w := tg.do("POST", "/api/projects/_/stories/oldbook/relations", admin,
		`{"a":"Tri Hạ","b":"Nhược Vy","after":"Phản bội","type":"enemy","reason":"trở mặt","fromChapter":4}`)
	if w.Code != 200 {
		t.Fatalf("typed relation: %d %s", w.Code, w.Body.String())
	}
	raw, err := os.ReadFile(filepath.Join(engine, "relationship_log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"relation":"Phản bội","type":"enemy"`) {
		t.Fatalf("type not stored: %s", raw)
	}
	brain := tg.do("GET", "/api/projects/_/stories/oldbook/brain", admin, "")
	if brain.Code != 200 {
		t.Fatalf("brain: %d %s", brain.Code, brain.Body.String())
	}
	var body naoBrainBody
	if err := json.Unmarshal(brain.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var phases []struct {
		FromChapter int    `json:"fromChapter"`
		Type        string `json:"type"`
		Label       string `json:"label"`
	}
	for _, edge := range body.Edges {
		if edge.A == "nv:tri-ha" && edge.B == "nv:nhUOC-vy" {
			phases = edge.Phases
		}
	}
	last := phases[len(phases)-1]
	if last.FromChapter != 4 || last.Type != "enemy" || last.Label != "Phản bội" {
		t.Fatalf("brain phase = %+v", last)
	}
	// Kiểu lạ bị từ chối, không ghi vào log.
	if code := tg.do("POST", "/api/projects/_/stories/oldbook/relations", admin,
		`{"a":"Tri Hạ","b":"Nhược Vy","after":"Mơ hồ","type":"bogus","fromChapter":5}`).Code; code != 400 {
		t.Fatalf("unknown type: %d", code)
	}
	after, err := os.ReadFile(filepath.Join(engine, "relationship_log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "Mơ hồ") {
		t.Fatalf("unknown type was stored: %s", after)
	}
}

func TestNaoRelationMarkQueuesRewrite(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	w := tg.do("POST", "/api/projects/_/stories/oldbook/relations", admin,
		`{"a":"Tri Hạ","b":"Nhược Vy","after":"Đồng minh","reason":"cứu nhau","fromChapter":2,"apply":"mark"}`)
	if w.Code != 200 {
		t.Fatalf("mark relation: %d %s", w.Code, w.Body.String())
	}
	var progress struct {
		PendingRewrites []int  `json:"pending_rewrites"`
		RewriteReason   string `json:"rewrite_reason"`
	}
	raw, err := os.ReadFile(filepath.Join(engine, "meta", "progress.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &progress); err != nil {
		t.Fatal(err)
	}
	if len(progress.PendingRewrites) != 2 || progress.PendingRewrites[0] != 2 || progress.PendingRewrites[1] != 3 {
		t.Fatalf("pending rewrites = %v", progress.PendingRewrites)
	}
	want := "Chỉ sửa câu có quan hệ Tri Hạ – Nhược Vy: “Đối đầu” → “Đồng minh”."
	if progress.RewriteReason != want {
		t.Fatalf("rewrite reason = %q, want %q", progress.RewriteReason, want)
	}
	// Không có apply thì 409 như xưng hô.
	if code := tg.do("POST", "/api/projects/_/stories/oldbook/relations", admin,
		`{"a":"Tri Hạ","b":"Nhược Vy","after":"Đồng minh","reason":"vì","fromChapter":1}`).Code; code != http.StatusConflict {
		t.Fatalf("relation without apply: %d", code)
	}
}

func TestNaoReaderCannotWrite(t *testing.T) {
	tg := newTestGateway(t)
	naoFixture(t, tg)
	reader := tg.login("reader", "reader-pass")

	if code := tg.do("GET", "/api/projects/_/stories/oldbook/address-terms", reader, "").Code; code != 200 {
		t.Fatalf("reader read: %d", code)
	}
	if code := tg.do("GET", "/api/projects/_/stories/oldbook/brain", reader, "").Code; code != 200 {
		t.Fatalf("reader brain: %d", code)
	}
	if code := tg.do("PUT", "/api/projects/_/stories/oldbook/address-terms", reader,
		`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":5}}`).Code; code != 403 {
		t.Fatalf("reader put: %d", code)
	}
	if code := tg.do("POST", "/api/projects/_/stories/oldbook/relations", reader,
		`{"a":"Tri Hạ","b":"Nhược Vy","after":"Đồng minh","fromChapter":5}`).Code; code != 403 {
		t.Fatalf("reader relation: %d", code)
	}
}

func TestNaoValidationRejectsBadInput(t *testing.T) {
	tg := newTestGateway(t)
	naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	cases := []struct {
		name string
		path string
		body string
		want string
	}{
		{"missing call", "/api/projects/_/stories/oldbook/address-terms",
			`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"","self":"tôi","fromChapter":5,"reason":"vì"}}`, "cách gọi"},
		{"newline in call", "/api/projects/_/stories/oldbook/address-terms",
			`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"chị\nem","self":"tôi","fromChapter":5,"reason":"vì"}}`, "một dòng"},
		{"unknown character", "/api/projects/_/stories/oldbook/address-terms",
			`{"term":{"from":"Người Lạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":5,"reason":"vì"}}`, "Không tìm thấy nhân vật"},
		{"same character", "/api/projects/_/stories/oldbook/address-terms",
			`{"term":{"from":"Tri Hạ","to":"Tri Hạ","call":"chị","self":"tôi","fromChapter":5,"reason":"vì"}}`, "chính mình"},
		{"bad fromChapter", "/api/projects/_/stories/oldbook/address-terms",
			`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":0,"reason":"vì"}}`, "lớn hơn 0"},
		{"bad apply", "/api/projects/_/stories/oldbook/address-terms",
			`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":5},"apply":"replace"}`, "áp dụng"},
		{"missing relation", "/api/projects/_/stories/oldbook/relations",
			`{"a":"Tri Hạ","b":"Nhược Vy","after":"","reason":"vì","fromChapter":5}`, "quan hệ mới"},
		{"relation same pair", "/api/projects/_/stories/oldbook/relations",
			`{"a":"Tri Hạ","b":"Tri Hạ","after":"Đồng minh","reason":"vì","fromChapter":5}`, "hai nhân vật khác nhau"},
		{"stale term id", "/api/projects/_/stories/oldbook/address-terms",
			`{"term":{"id":"nv:tri-ha|nv:nhUOC-vy|7","from":"Tri Hạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":5,"reason":"vì"}}`, "không còn tồn tại"},
		{"forward renamed to next", "/api/projects/_/stories/oldbook/address-terms",
			`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":5,"reason":"vì"},"apply":"forward"}`, "áp dụng"},
	}
	for _, tc := range cases {
		method := http.MethodPut
		if strings.HasSuffix(tc.path, "/relations") {
			method = http.MethodPost
		}
		w := tg.do(method, tc.path, admin, tc.body)
		if w.Code != 400 {
			t.Fatalf("%s: %d %s", tc.name, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), tc.want) {
			t.Fatalf("%s: body %s, want %q", tc.name, w.Body.String(), tc.want)
		}
	}
}

// TestNaoWriteRequiresReason: sửa xưng hô và quan hệ trên web phải có lý do (Câu 21);
// thiếu lý do là 400 kèm field để form mở đúng ô, và không ghi gì xuống đĩa.
func TestNaoWriteRequiresReason(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	cases := []struct {
		name, path, body string
	}{
		{"term", "/api/projects/_/stories/oldbook/address-terms",
			`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":5}}`},
		{"term blank reason", "/api/projects/_/stories/oldbook/address-terms",
			`{"term":{"from":"Tri Hạ","to":"Nhược Vy","call":"chị","self":"tôi","fromChapter":5,"reason":"   "}}`},
		{"relation", "/api/projects/_/stories/oldbook/relations",
			`{"a":"Tri Hạ","b":"Nhược Vy","after":"Thù địch","fromChapter":5}`},
	}
	for _, tc := range cases {
		method := http.MethodPut
		if strings.HasSuffix(tc.path, "/relations") {
			method = http.MethodPost
		}
		w := tg.do(method, tc.path, admin, tc.body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", tc.name, w.Code, w.Body.String())
		}
		var body struct {
			Error string `json:"error"`
			Field string `json:"field"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Field != "reason" || body.Error == "" {
			t.Fatalf("%s: body %s, want field=reason", tc.name, w.Body.String())
		}
	}
	raw, err := os.ReadFile(filepath.Join(engine, "address_terms.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "chị") {
		t.Fatalf("400 must not write: %s", raw)
	}
	log, err := os.ReadFile(filepath.Join(engine, "relationship_log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(log), "Thù địch") {
		t.Fatalf("400 must not append relation: %s", log)
	}
}

// TestNaoExpiredTermNotEditable: dòng xưng hô đã hết hiệu lực là lịch sử (Câu 21),
// sửa vào nó là 409 và không sinh dòng mới.
func TestNaoExpiredTermNotEditable(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	// Fixture: Nhược Vy → Kỳ Dương từ ch.1, đóng sau ch.2.
	const id = "nv:nhUOC-vy|nv:ky-duong|1"
	w := tg.do("PUT", "/api/projects/_/stories/oldbook/address-terms", admin,
		`{"term":{"id":"`+id+`","from":"Nhược Vy","to":"Kỳ Dương","call":"em","self":"anh","fromChapter":5,"reason":"đổi"}}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("expired row edit: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hết hiệu lực") {
		t.Fatalf("body = %s", w.Body.String())
	}
	raw, err := os.ReadFile(filepath.Join(engine, "address_terms.json"))
	if err != nil {
		t.Fatal(err)
	}
	var terms []struct {
		FromChapter int `json:"from_chapter"`
	}
	if err := json.Unmarshal(raw, &terms); err != nil {
		t.Fatal(err)
	}
	if len(terms) != 2 {
		t.Fatalf("409 must not add a row: %s", raw)
	}
}

type naoBrainBody struct {
	Chapters []struct {
		N      int    `json:"n"`
		Title  string `json:"title"`
		Status string `json:"status"`
	} `json:"chapters"`
	Entities []struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		Name        string `json:"name"`
		Role        string `json:"role"`
		FromChapter int    `json:"fromChapter"`
		Appears     []int  `json:"appears"`
	} `json:"entities"`
	Edges []struct {
		A      string `json:"a"`
		B      string `json:"b"`
		Phases []struct {
			FromChapter int    `json:"fromChapter"`
			Type        string `json:"type"`
			Label       string `json:"label"`
		} `json:"phases"`
	} `json:"edges"`
}

func TestNaoBrainRead(t *testing.T) {
	tg := newTestGateway(t)
	naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	w := tg.do("GET", "/api/projects/_/stories/oldbook/brain", admin, "")
	if w.Code != 200 {
		t.Fatalf("brain: %d %s", w.Code, w.Body.String())
	}
	var body naoBrainBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	if len(body.Chapters) != 8 {
		t.Fatalf("chapters = %+v", body.Chapters)
	}
	if body.Chapters[0].Status != "written" || body.Chapters[3].Status != "unwritten" || body.Chapters[3].Title != "Vỡ" {
		t.Fatalf("chapter statuses = %+v", body.Chapters)
	}
	byID := map[string]naoBrainEntityView{}
	for _, entity := range body.Entities {
		byID[entity.ID] = naoBrainEntityView{Type: entity.Type, Name: entity.Name, From: entity.FromChapter, Appears: entity.Appears, Role: entity.Role}
	}
	triHa, ok := byID["nv:tri-ha"]
	if !ok {
		t.Fatalf("missing character entity: %+v", byID)
	}
	if triHa.Type != "char" || triHa.From != 1 || len(triHa.Appears) != 3 || triHa.Appears[2] != 3 {
		t.Fatalf("Tri Hạ entity = %+v", triHa)
	}
	fs, ok := byID["fs:khan-do"]
	if !ok || fs.Type != "fs" || fs.From != 2 || len(fs.Appears) != 1 || fs.Appears[0] != 2 {
		t.Fatalf("foreshadow entity = %+v", byID)
	}
	if ev, ok := byID["ev:2-0"]; !ok || ev.Type != "ev" || ev.From != 2 {
		t.Fatalf("event entity = %+v", byID)
	}
	if rule, ok := byID["rule:0"]; !ok || rule.Type != "rule" || rule.From != 1 {
		t.Fatalf("rule entity = %+v", byID)
	}
	if len(body.Edges) != 2 {
		t.Fatalf("edges = %+v", body.Edges)
	}
	var pair naoBrainEdgeView
	for _, edge := range body.Edges {
		if edge.A == "nv:tri-ha" && edge.B == "nv:nhUOC-vy" {
			pair = naoBrainEdgeView{Phases: edge.Phases}
		}
	}
	if len(pair.Phases) != 2 {
		t.Fatalf("pair phases = %+v", pair.Phases)
	}
	if pair.Phases[0].FromChapter != 1 || pair.Phases[0].Label != "Bạn diễn cũ" || pair.Phases[1].FromChapter != 2 || pair.Phases[1].Label != "Đối đầu" {
		t.Fatalf("phases not ordered: %+v", pair.Phases)
	}
	if pair.Phases[0].Type != "rel" {
		t.Fatalf("phase type = %q", pair.Phases[0].Type)
	}
	if pair.Phases[1].Type != "enemy" {
		t.Fatalf("typed phase = %+v", pair.Phases[1])
	}
	var ally naoBrainEdgeView
	for _, edge := range body.Edges {
		if edge.A == "nv:tri-ha" && edge.B == "nv:ky-duong" {
			ally = naoBrainEdgeView{Phases: edge.Phases}
		}
	}
	if len(ally.Phases) != 1 || ally.Phases[0].Type != "ally" {
		t.Fatalf("ally phases = %+v", ally.Phases)
	}
}

type naoBrainEntityView struct {
	Type    string
	Name    string
	Role    string
	From    int
	Appears []int
}

type naoBrainEdgeView struct {
	Phases []struct {
		FromChapter int    `json:"fromChapter"`
		Type        string `json:"type"`
		Label       string `json:"label"`
	} `json:"phases"`
}

func TestNaoBrainChapterRewritingStatus(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	if err := os.WriteFile(filepath.Join(engine, "meta", "progress.json"),
		[]byte(`{"phase":"writing","current_chapter":4,"total_chapters":8,"completed_chapters":[1,2,3],"flow":"rewriting","pending_rewrites":[2],"in_progress_chapter":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	w := tg.do("GET", "/api/projects/_/stories/oldbook/brain", admin, "")
	if w.Code != 200 {
		t.Fatalf("brain: %d %s", w.Code, w.Body.String())
	}
	var body naoBrainBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, chapter := range body.Chapters {
		switch chapter.N {
		case 1:
			if chapter.Status != "written" {
				t.Fatalf("ch.1 status = %q", chapter.Status)
			}
		case 2:
			if chapter.Status != "rewriting" {
				t.Fatalf("ch.2 status = %q", chapter.Status)
			}
		}
	}
}

// TestNaoBrainChapterWritingStatus: engine viết chương 4 lần đầu thì thanh chương báo
// "writing" (Câu 23), không phải "unwritten" và không phải "rewriting".
func TestNaoBrainChapterWritingStatus(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	if err := os.WriteFile(filepath.Join(engine, "meta", "progress.json"),
		[]byte(`{"phase":"writing","current_chapter":4,"total_chapters":8,"completed_chapters":[1,2,3],"flow":"writing","in_progress_chapter":4}`), 0o644); err != nil {
		t.Fatal(err)
	}
	w := tg.do("GET", "/api/projects/_/stories/oldbook/brain", admin, "")
	if w.Code != 200 {
		t.Fatalf("brain: %d %s", w.Code, w.Body.String())
	}
	var body naoBrainBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	got := map[int]string{}
	for _, chapter := range body.Chapters {
		got[chapter.N] = chapter.Status
	}
	if got[3] != "written" || got[4] != "writing" || got[5] != "unwritten" {
		t.Fatalf("chapter statuses = %+v", got)
	}
}

// naoWriteRecord ghi bản ghi nghiệm thu một chương bằng đúng kho của engine, để Não
// truyện đọc facts.places/objects như dữ liệu thật.
func naoWriteRecord(t *testing.T, engine string, chapter int, facts domain.ChapterFacts) {
	t.Helper()
	s := store.NewStore(engine)
	if _, err := s.ChapterRecords.Accept(chapter, domain.ChapterOriginGenerated,
		fmt.Sprintf("Chính văn chương %d.", chapter), facts, domain.StyleDelta{}); err != nil {
		t.Fatalf("ghi bản ghi chương %d: %v", chapter, err)
	}
}

// TestNaoBrainPlaceAndObjectEntities: chapterfacts places[]/objects[] (GAP-49) thành
// thực thể place/obj với mã ổn định qua chương, và with[] thành cạnh "Liên quan" bắt
// đầu đúng chương. Chương nào chưa có dữ kiện thì không sinh thực thể loại đó.
func TestNaoBrainPlaceAndObjectEntities(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")

	naoWriteRecord(t, engine, 1, domain.ChapterFacts{
		Title: "Mở màn", Summary: "Hai người gặp nhau", Characters: []string{"Tri Hạ", "Nhược Vy"},
		KeyEvents: []string{"Gặp ở trường quay"},
		Places: []domain.FactEntity{{
			Name: "Trường quay số ba", Desc: "Nơi quay buổi đọc thoại.",
			With: []string{"Tri Hạ", "Nhược Vy"},
		}},
		Objects: []domain.FactEntity{{
			Name: "Kịch bản gốc", Desc: "Bản Tri Hạ viết, đứng tên Nhược Vy.",
			With: []string{"Nhược Vy"},
		}},
	})
	// Chương 2 nhắc lại địa điểm cũ, không nhắc lại vật: địa điểm gộp một nút, vật chỉ ở ch.1.
	naoWriteRecord(t, engine, 2, domain.ChapterFacts{
		Title: "Đối đầu", Summary: "Căng thẳng", Characters: []string{"Tri Hạ", "Nhược Vy", "Kỳ Dương"},
		KeyEvents: []string{"Đối đầu"},
		Places:    []domain.FactEntity{{Name: "Trường quay số ba", FirstChapter: 1, With: []string{"Kỳ Dương"}}},
	})

	w := tg.do("GET", "/api/projects/_/stories/oldbook/brain", admin, "")
	if w.Code != 200 {
		t.Fatalf("brain: %d %s", w.Code, w.Body.String())
	}
	var body naoBrainDetailBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	placeID := "dd:truong-quay-so-ba"
	i, ok := naoDetailEntity(t, body, placeID)
	if !ok {
		t.Fatalf("thiếu thực thể địa điểm: %+v", body.Entities)
	}
	place := body.Entities[i]
	if place.Type != "place" || place.FromChapter != 1 || strings.Join(intsText(place.Appears), ",") != "1,2" {
		t.Fatalf("địa điểm = %+v", place)
	}
	objectIdx, ok := naoDetailEntity(t, body, "vat:kich-ban-goc")
	if !ok {
		t.Fatalf("thiếu thực thể vật: %+v", body.Entities)
	}
	if object := body.Entities[objectIdx]; object.Type != "obj" || strings.Join(intsText(object.Appears), ",") != "1" {
		t.Fatalf("vật = %+v", object)
	}

	// with[] thành cạnh kiểu rel đúng chương, nhãn nhóm Liên quan.
	phasesOf := func(a, b string) []string {
		for _, edge := range body.Edges {
			if (edge.A == a && edge.B == b) || (edge.A == b && edge.B == a) {
				out := make([]string, 0, len(edge.Phases))
				for _, phase := range edge.Phases {
					out = append(out, fmt.Sprintf("%d:%s:%s", phase.FromChapter, phase.Type, phase.Label))
				}
				return out
			}
		}
		return nil
	}
	if got := phasesOf(placeID, "nv:tri-ha"); strings.Join(got, "|") != "1:rel:Liên quan" {
		t.Fatalf("cạnh địa điểm–Tri Hạ = %v", got)
	}
	if got := phasesOf(placeID, "nv:ky-duong"); strings.Join(got, "|") != "2:rel:Liên quan" {
		t.Fatalf("cạnh địa điểm–Kỳ Dương = %v", got)
	}
	if got := phasesOf("vat:kich-ban-goc", "nv:nhUOC-vy"); strings.Join(got, "|") != "1:rel:Liên quan" {
		t.Fatalf("cạnh vật–Nhược Vy = %v", got)
	}

	// upTo=1: thấy địa điểm và cạnh ch.1, không thấy cạnh ch.2.
	one := naoBrainDetail(t, tg.do("GET", "/api/projects/_/stories/oldbook/brain?upTo=1", admin, ""))
	if i, ok := naoDetailEntity(t, one, placeID); !ok || strings.Join(intsText(one.Entities[i].Appears), ",") != "1" {
		t.Fatalf("upTo=1 địa điểm = %+v", one.Entities)
	}
	for _, edge := range one.Edges {
		if edge.A == placeID || edge.B == placeID {
			for _, phase := range edge.Phases {
				if phase.FromChapter > 1 {
					t.Fatalf("upTo=1 lộ cạnh ch.%d", phase.FromChapter)
				}
			}
		}
	}
}

// ── M8: ?upTo=N cắt dữ kiện Não truyện theo chương đang đọc ──

type naoBrainDetailBody struct {
	Entities []struct {
		ID          string   `json:"id"`
		Type        string   `json:"type"`
		Name        string   `json:"name"`
		Desc        string   `json:"desc"`
		FromChapter int      `json:"fromChapter"`
		Appears     []int    `json:"appears"`
		Aliases     []string `json:"aliases"`
		Timeline    []struct {
			Chapter int    `json:"chapter"`
			Text    string `json:"text"`
		} `json:"timeline"`
	} `json:"entities"`
	Edges []struct {
		A      string `json:"a"`
		B      string `json:"b"`
		Phases []struct {
			FromChapter int    `json:"fromChapter"`
			Type        string `json:"type"`
			Label       string `json:"label"`
		} `json:"phases"`
	} `json:"edges"`
}

func naoBrainDetail(t *testing.T, w interface {
	Result() *http.Response
}) naoBrainDetailBody {
	t.Helper()
	body, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	var out naoBrainDetailBody
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	return out
}

func naoDetailEntity(t *testing.T, body naoBrainDetailBody, id string) (int, bool) {
	t.Helper()
	for i, entity := range body.Entities {
		if entity.ID == id {
			return i, true
		}
	}
	return -1, false
}

func naoTimeline(t *testing.T, body naoBrainDetailBody, id string) []string {
	t.Helper()
	i, ok := naoDetailEntity(t, body, id)
	if !ok {
		t.Fatalf("missing entity %q", id)
	}
	out := make([]string, 0, len(body.Entities[i].Timeline))
	for _, moment := range body.Entities[i].Timeline {
		out = append(out, fmt.Sprintf("%d:%s", moment.Chapter, moment.Text))
	}
	return out
}

// TestNaoBrainUpToTrimsToReadingChapter: người đọc ở chương N chỉ thấy những gì đã
// xảy ra tới N — thực thể, mốc xuất hiện, pha quan hệ và diễn biến sau N đều bị cắt.
func TestNaoBrainUpToTrimsToReadingChapter(t *testing.T) {
	tg := newTestGateway(t)
	engine := naoFixture(t, tg)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	// Biệt danh (Não identity) + một lần đổi cách gọi ở ch.2 để dựng "Diễn biến".
	if err := os.WriteFile(filepath.Join(engine, "characters.json"), []byte(`[
  {"id":"nv:tri-ha","name":"Tri Hạ","aliases":["Hạ","chị Hạ"],"role":"protagonist","description":"Người kể."},
  {"id":"nv:nhUOC-vy","name":"Nhược Vy","role":"antagonist","description":"Phản diện."},
  {"id":"nv:ky-duong","name":"Kỳ Dương","role":"supporting","description":"Bạn diễn."}
]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(engine, "address_terms.json"), []byte(`[
  {"from":"nv:tri-ha","to":"nv:nhUOC-vy","calls":"em","self":"anh","from_chapter":1,"reason":"ch.1","scope":"truyen","source":"chương 1"},
  {"from":"nv:tri-ha","to":"nv:nhUOC-vy","calls":"cô Vy","self":"tôi","from_chapter":2,"reason":"đổi ở ch.2","scope":"truyen","source":"chương 2"}
]`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Không có upTo: đủ dữ kiện (màn Não truyện của người viết).
	full := naoBrainDetail(t, tg.do("GET", "/api/projects/_/stories/oldbook/brain", admin, ""))
	if len(full.Entities) != 6 {
		t.Fatalf("full entities = %d", len(full.Entities))
	}
	if got := naoTimeline(t, full, "nv:tri-ha"); strings.Join(got, "|") !=
		"1:Bạn diễn cũ · Nhược Vy|2:Đồng minh · Kỳ Dương|2:Đổi thành Đối đầu · Nhược Vy|2:Tri Hạ gọi Nhược Vy là “cô Vy”" {
		t.Fatalf("full timeline = %v", got)
	}
	if i, _ := naoDetailEntity(t, full, "nv:tri-ha"); strings.Join(full.Entities[i].Aliases, ",") != "Hạ,chị Hạ" {
		t.Fatalf("aliases = %v", full.Entities[i].Aliases)
	}
	if i, _ := naoDetailEntity(t, full, "rule:0"); full.Entities[i].Appears == nil || full.Entities[i].Aliases == nil {
		t.Fatal("appears/aliases của quy tắc phải là mảng, không null")
	}

	// upTo=1: chỉ những gì đã xảy ra ở chương 1.
	one := naoBrainDetail(t, tg.do("GET", "/api/projects/_/stories/oldbook/brain?upTo=1", reader, ""))
	for _, id := range []string{"nv:tri-ha", "nv:nhUOC-vy", "rule:0"} {
		if _, ok := naoDetailEntity(t, one, id); !ok {
			t.Fatalf("upTo=1 thiếu %q: %+v", id, one.Entities)
		}
	}
	for _, id := range []string{"nv:ky-duong", "fs:khan-do", "ev:2-0"} {
		if _, ok := naoDetailEntity(t, one, id); ok {
			t.Fatalf("upTo=1 lộ thực thể sau chương: %q", id)
		}
	}
	if i, _ := naoDetailEntity(t, one, "nv:tri-ha"); strings.Join(intsText(one.Entities[i].Appears), ",") != "1" {
		t.Fatalf("upTo=1 appears = %v", one.Entities[i].Appears)
	}
	if got := naoTimeline(t, one, "nv:tri-ha"); strings.Join(got, "|") != "1:Bạn diễn cũ · Nhược Vy" {
		t.Fatalf("upTo=1 timeline = %v", got)
	}
	if len(one.Edges) != 1 || one.Edges[0].A != "nv:tri-ha" || one.Edges[0].B != "nv:nhUOC-vy" {
		t.Fatalf("upTo=1 edges = %+v", one.Edges)
	}
	if len(one.Edges[0].Phases) != 1 || one.Edges[0].Phases[0].FromChapter != 1 {
		t.Fatalf("upTo=1 phases = %+v", one.Edges[0].Phases)
	}

	// upTo=2: đủ hai pha của cặp chính, có Kỳ Dương, không có gì sau ch.3.
	two := naoBrainDetail(t, tg.do("GET", "/api/projects/_/stories/oldbook/brain?upTo=2", reader, ""))
	if _, ok := naoDetailEntity(t, two, "nv:ky-duong"); !ok {
		t.Fatal("upTo=2 phải thấy Kỳ Dương (có từ ch.2)")
	}
	if i, _ := naoDetailEntity(t, two, "nv:tri-ha"); strings.Join(intsText(two.Entities[i].Appears), ",") != "1,2" {
		t.Fatalf("upTo=2 appears = %v", two.Entities[i].Appears)
	}
	if got := naoTimeline(t, two, "nv:nhUOC-vy"); strings.Join(got, "|") != "1:Bạn diễn cũ · Tri Hạ|2:Đổi thành Đối đầu · Tri Hạ" {
		t.Fatalf("upTo=2 timeline = %v", got)
	}
	if len(two.Edges) != 2 {
		t.Fatalf("upTo=2 edges = %+v", two.Edges)
	}

	// upTo vượt số chương = không cắt gì.
	beyond := naoBrainDetail(t, tg.do("GET", "/api/projects/_/stories/oldbook/brain?upTo=99", reader, ""))
	if len(beyond.Entities) != len(full.Entities) || len(beyond.Edges) != len(full.Edges) {
		t.Fatalf("upTo=99 cắt mất dữ kiện: %d/%d", len(beyond.Entities), len(beyond.Edges))
	}
}

func intsText(values []int) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, strconv.Itoa(v))
	}
	return out
}

// TestNaoBrainUpToRejectsInvalid: upTo không phải số chương dương → 400; bỏ trống = không cắt.
func TestNaoBrainUpToRejectsInvalid(t *testing.T) {
	tg := newTestGateway(t)
	naoFixture(t, tg)
	reader := tg.login("reader", "reader-pass")

	for _, raw := range []string{"0", "-1", "abc", "1.5", "%20"} {
		w := tg.do("GET", "/api/projects/_/stories/oldbook/brain?upTo="+raw, reader, "")
		if raw == "%20" {
			// Khoảng trắng = không truyền: đọc bình thường.
			if w.Code != 200 {
				t.Fatalf("upTo trống: %d %s", w.Code, w.Body.String())
			}
			continue
		}
		if w.Code != 400 || !strings.Contains(w.Body.String(), naoUpToError) {
			t.Fatalf("upTo=%q: %d %s", raw, w.Code, w.Body.String())
		}
	}
}
