package lessons

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// legacyLesson builds an entry shaped like the file written before levels stopped
// splitting a lesson (and before §M split the counts): the id is the legacy
// severity-split hash and evidence is {"count","kind"} — a raw map because the
// Go struct no longer marshals that shape.
func legacyLesson(pack, ref, level, state string, createdAt time.Time) map[string]any {
	out := map[string]any{
		"id": legacyID(pack, ref, level), "origin": "review", "refs": []string{ref},
		"text":     "bài học mức " + level,
		"evidence": map[string]any{"count": 2, "kind": level, "stories": 2, "pack": "", "packsCount": 1},
		"scope":    "pack", "packId": pack, "packName": "Gói A", "state": state, "createdAt": createdAt,
	}
	return out
}

func writeLessons(t *testing.T, dir string, lessons []map[string]any) {
	t.Helper()
	if err := store.NewIO(dir).WriteJSON("lessons.json", map[string]any{"lessons": lessons}); err != nil {
		t.Fatal(err)
	}
}

func readLessonsFile(t *testing.T, dir string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "lessons.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Câu 11: id cũ tách theo mức được gộp về id nhóm, quyết định được giữ, id lạ bị xoá và tệp
// được ghi lại kèm `schema: 2`; chạy lần hai không ghi gì.
func TestStoreMigratesSeveritySplitDecisions(t *testing.T) {
	const pack, ref = "nu-tan-bao-thu", "Bộ A §4.3"
	start := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	decided := start.Add(48 * time.Hour)
	dir := t.TempDir()

	pending := legacyLesson(pack, ref, "warning", "pending", start)
	kept := legacyLesson(pack, ref, "error", "kept", start.Add(time.Hour))
	kept["decidedAt"] = decided
	stranger := legacyLesson(pack, "Master §9", "error", "pending", start)
	stranger["id"] = "lesson-0123456789abcdef01234567"
	writeLessons(t, dir, []map[string]any{pending, kept, stranger})

	s := NewStore(dir)
	c, err := s.read()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Lessons) != 1 {
		t.Fatalf("migrated %d lessons; need the merged group alone: %+v", len(c.Lessons), c.Lessons)
	}
	got := c.Lessons[0]
	if want := ID(GroupKey(pack, ref)); got.ID != want {
		t.Fatalf("legacy ids merged onto %q; need the group id %q", got.ID, want)
	}
	if got.State != "kept" {
		t.Fatalf("merged decision %q; kept must win over pending", got.State)
	}
	if got.DecidedAt == nil || !got.DecidedAt.Equal(decided) {
		t.Fatalf("merged decidedAt %v; need the kept decision's %v", got.DecidedAt, decided)
	}
	if !got.CreatedAt.Equal(start) {
		t.Fatalf("merged createdAt %v; need the earliest %v", got.CreatedAt, start)
	}
	if got.Text != kept["text"] {
		t.Fatalf("merged text %q; need the winning entry's %q", got.Text, kept["text"])
	}

	var onDisk struct {
		Schema  int      `json:"schema"`
		Lessons []Lesson `json:"lessons"`
	}
	raw := readLessonsFile(t, dir)
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk.Schema != 2 {
		t.Fatalf("lessons.json schema = %d; need 2 (%s)", onDisk.Schema, raw)
	}
	if len(onDisk.Lessons) != 1 {
		t.Fatalf("lessons.json kept %d entries; need 1: %s", len(onDisk.Lessons), raw)
	}

	c, err = s.read()
	if err != nil {
		t.Fatal(err)
	}
	if c.Schema != 2 || len(c.Lessons) != 1 {
		t.Fatalf("second run changed the catalog: %+v", c)
	}
	if after := readLessonsFile(t, dir); string(after) != string(raw) {
		t.Fatalf("second run rewrote lessons.json:\nbefore %s\nafter  %s", raw, after)
	}
}

// Câu 11: cặp đã bỏ + chờ gộp thành “đã bỏ”, giữ decidedAt của lần bỏ.
func TestStoreMigrationKeepsDroppedOverPending(t *testing.T) {
	const pack, ref = "nguoc-hon-nhan", "Master §3"
	start := time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)
	droppedAt := start.Add(24 * time.Hour)
	dir := t.TempDir()

	dropped := legacyLesson(pack, ref, "error", "dropped", start.Add(time.Hour))
	dropped["decidedAt"] = droppedAt
	pending := legacyLesson(pack, ref, "warning", "pending", start)
	writeLessons(t, dir, []map[string]any{pending, dropped})

	c, err := NewStore(dir).read()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Lessons) != 1 {
		t.Fatalf("dropped+pending pair produced %d lessons: %+v", len(c.Lessons), c.Lessons)
	}
	got := c.Lessons[0]
	if got.ID != ID(GroupKey(pack, ref)) || got.State != "dropped" || got.DecidedAt == nil || !got.DecidedAt.Equal(droppedAt) {
		t.Fatalf("dropped must win over pending: %+v", got)
	}
	if got.Scope != "pack" {
		t.Fatalf("a decided shared ref keeps the scope the user chose: %+v", got)
	}
}

// Câu 11 + câu 15: mục dùng chung còn chờ duyệt sau khi gộp về đúng phạm vi mặc định “Mọi gói”;
// mục đã quyết giữ phạm vi người dùng chọn; mục `Bộ …` không bị chạm.
func TestStoreMigrationScopesPendingSharedRefs(t *testing.T) {
	const pack = "nu-tan-bao-thu"
	start := time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)
	dir := t.TempDir()

	sharedPendingError := legacyLesson(pack, "Master §3", "error", "pending", start)
	sharedPendingWarning := legacyLesson(pack, "Master §3", "warning", "pending", start.Add(time.Hour))
	keptAt := start.Add(2 * time.Hour)
	sharedKept := legacyLesson(pack, "Master §11", "error", "kept", start)
	sharedKept["decidedAt"] = keptAt
	packPendingError := legacyLesson(pack, "Bộ A §4.3", "error", "pending", start)
	packPendingWarning := legacyLesson(pack, "Bộ A §4.3", "warning", "pending", start.Add(time.Hour))
	writeLessons(t, dir, []map[string]any{sharedPendingError, sharedPendingWarning, sharedKept, packPendingError, packPendingWarning})

	c, err := NewStore(dir).read()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Lessons) != 3 {
		t.Fatalf("migrated %d lessons; need shared pending, shared kept and Bộ: %+v", len(c.Lessons), c.Lessons)
	}
	byRef := map[string]Lesson{}
	for _, l := range c.Lessons {
		byRef[l.Refs[0]] = l
	}
	if got := byRef["Master §3"]; got.State != "pending" || got.Scope != "all" {
		t.Fatalf("pending shared ref must default to Mọi gói: %+v", got)
	}
	if got := byRef["Master §11"]; got.State != "kept" || got.Scope != "pack" {
		t.Fatalf("decided shared ref must keep its chosen scope: %+v", got)
	}
	if got := byRef["Bộ A §4.3"]; got.State != "pending" || got.Scope != "pack" {
		t.Fatalf("pack ref must stay in its pack: %+v", got)
	}
}

// TestLoadChipsNeverNull pins the wire contract: a catalog written by hand (or an
// older version) without evidence.chips must still marshal as "chips": [] —
// the frontend maps the array without a nil check.
func TestLoadChipsNeverNull(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lessons.json"),
		[]byte(`{"schema":2,"lessons":[{"id":"lesson-x","origin":"user","refs":["Xưng hô"],"text":"Đổi cách gọi ngay.","evidence":{"userEdits":1,"object":"xưng hô","stories":1,"pack":"","packsCount":1},"scope":"all","packId":"","packName":"","state":"pending","createdAt":"2026-10-01T00:00:00Z"}]}`),
		0o644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(dir)
	c, err := s.read()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Lessons) != 1 || c.Lessons[0].Evidence.Chips == nil {
		t.Fatalf("chips phải là []: %+v", c.Lessons)
	}
	out, err := json.Marshal(c.Lessons[0].Evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`"chips":[]`)) {
		t.Fatalf("evidence marshal ra chips khác []: %s", out)
	}
}

// §M: bản ghi cũ không được chuyển dữ liệu — số đếm được suy ra khi đọc từ
// `count` + `kind`: kind là mức thì thành errors/warnings, kind là danh từ thì
// thành userEdits + object. File trên đĩa giữ nguyên dạng cũ.
func TestLoadDerivesLegacyEvidence(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lessons.json"),
		[]byte(`{"schema":2,"lessons":[
			{"id":"l-e","origin":"review","refs":["Master §7"],"text":"a","evidence":{"count":2,"kind":"error","stories":2,"pack":"","packsCount":1},"scope":"all","packId":"p","packName":"P","state":"pending","createdAt":"2026-10-01T00:00:00Z"},
			{"id":"l-w","origin":"review","refs":["Master §8"],"text":"b","evidence":{"count":4,"kind":"warning","stories":3,"pack":"","packsCount":2},"scope":"all","packId":"p","packName":"P","state":"pending","createdAt":"2026-10-01T00:00:00Z"},
			{"id":"l-u","origin":"user","refs":["Xưng hô"],"text":"c","evidence":{"count":3,"kind":"xưng hô","stories":2,"pack":"","packsCount":2},"scope":"all","packId":"p","packName":"P","state":"pending","createdAt":"2026-10-01T00:00:00Z"}
		]}`),
		0o644); err != nil {
		t.Fatal(err)
	}
	c, err := NewStore(dir).read()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Evidence{}
	for _, l := range c.Lessons {
		byID[l.ID] = l.Evidence
	}
	if e := byID["l-e"]; e.Errors != 2 || e.Warnings != 0 || e.UserEdits != 0 || e.Object != "" {
		t.Fatalf("kind=error phải thành errors: %+v", e)
	}
	if e := byID["l-w"]; e.Warnings != 4 || e.Errors != 0 || e.UserEdits != 0 {
		t.Fatalf("kind=warning phải thành warnings: %+v", e)
	}
	if e := byID["l-u"]; e.UserEdits != 3 || e.Object != "xưng hô" || e.Errors != 0 || e.Warnings != 0 {
		t.Fatalf("kind=danh từ phải thành userEdits + object: %+v", e)
	}
	raw := readLessonsFile(t, dir)
	if !bytes.Contains(raw, []byte(`"count":3`)) || !bytes.Contains(raw, []byte(`"kind":"xưng hô"`)) {
		t.Fatalf("§M không chuyển dữ liệu — file phải giữ dạng cũ: %s", raw)
	}
}

// §M: chip loại bám `origin`, không đổi khi Sync đếm lại bằng chứng — một bài
// `origin:"user"` có id trùng nhóm của ứng viên từ lần chấm vẫn giữ origin
// "user" sau khi evidence bị làm mới.
func TestSyncKeepsOriginOnRecount(t *testing.T) {
	dir := t.TempDir()
	id := ID(GroupKey("pack-a", "Master §7"))
	seed, err := json.Marshal(map[string]any{
		"schema": 2,
		"lessons": []map[string]any{{
			"id": id, "origin": "user", "refs": []string{"Master §7"},
			"text":     "Góp ý của người dùng.",
			"evidence": map[string]any{"count": 2, "kind": "kết chương", "stories": 2, "pack": "", "packsCount": 1},
			"scope":    "all", "packId": "pack-a", "packName": "Gói A", "state": "pending",
			"createdAt": "2026-10-01T00:00:00Z",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lessons.json"), seed, 0o644); err != nil {
		t.Fatal(err)
	}
	list, err := NewStore(dir).Sync(Aggregate(observations(2, "error")))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("sync produced %d lessons: %+v", len(list), list)
	}
	got := list[0]
	if got.Origin != "user" {
		t.Fatalf("đếm lại đổi origin thành %q — chip phải bám người tạo", got.Origin)
	}
	if got.Evidence.Errors != 2 || got.Evidence.UserEdits != 0 || got.Evidence.Object != "" {
		t.Fatalf("evidence phải phản ánh đợt quét mới: %+v", got.Evidence)
	}
}
