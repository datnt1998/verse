package web

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/ledger"
	"github.com/datnt1998/verse/apps/noverse/internal/lessons"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// Bộ golden của web/mock: các endpoint mà mock từng tự tính (sổ cái, bài học,
// vũ trụ, chất lượng, vi phạm quy tắc) giờ phát lại đúng output của handler Go.
// `go test -update` ghi lại web/mock/golden/ sau khi chủ đích đổi handler hoặc
// fixture; lần chạy thường lỗi khi golden cũ.

var updateMockGoldens = flag.Bool("update", false, "ghi lại golden của mock dưới web/mock/golden")

func mockGoldenPath(t *testing.T, name string) string {
	t.Helper()
	// Test chạy với cwd = thư mục gói (internal/entry/web) → gốc repo là ../../..
	return filepath.Join("..", "..", "..", "web", "mock", "golden", name)
}

// assertMockGolden so body JSON của handler với golden đã chốt (so sánh dạng đã
// in đẹp; thứ tự khoá = thứ tự marshal của handler). -update ghi đè file.
func assertMockGolden(t *testing.T, name string, body []byte) {
	t.Helper()
	assertGoldenBody(t, name, body, true)
}

// assertGoldenPinned so response handler với golden của một package khác (do
// internal/lessons và internal/ledger ghi). Read-only: -update KHÔNG ghi đè —
// nếu không, hai package song song sẽ ghi đua nhau vào một file.
func assertGoldenPinned(t *testing.T, name string, body []byte) {
	t.Helper()
	assertGoldenBody(t, name, body, false)
}

func assertGoldenBody(t *testing.T, name string, body []byte, writable bool) {
	t.Helper()
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		t.Fatalf("golden %s: response không phải JSON: %v\n%s", name, err, body)
	}
	// Body của Encoder.Encode đã có newline cuối; các file ghi bởi
	// internal/lessons và internal/ledger (Marshal + Indent) chỉ một — chuẩn
	// hoá về đúng một newline để hai lớp so byte được với nhau.
	for bytes.HasSuffix(pretty.Bytes(), []byte("\n\n")) {
		pretty.Truncate(pretty.Len() - 1)
	}
	if !bytes.HasSuffix(pretty.Bytes(), []byte("\n")) {
		pretty.WriteByte('\n')
	}
	path := mockGoldenPath(t, name)
	if writable && *updateMockGoldens {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (chạy go test -update ở package chủ để tạo)", path, err)
	}
	if !bytes.Equal(pretty.Bytes(), want) {
		t.Fatalf("%s lệch output handler; nếu chủ đích, chạy `go test ./... -update`\ngot:\n%s", path, pretty.String())
	}
}

// goldenStoryDir tạo một thư mục truyện tối thiểu trong dự án: story.json + cấu
// hình gói thể loại. Trả về thư mục truyện.
func goldenStoryDir(t *testing.T, project, name, title, pack string) string {
	t.Helper()
	dir := filepath.Join(project, "kich_ban", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := library.WriteStoryFile(dir, library.StoryFile{Title: title}); err != nil {
		t.Fatal(err)
	}
	if pack != "" {
		if err := store.NewIO(dir).WriteJSON(".ainovel/config.json", map[string]string{"style": pack}); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// goldenReviewCheckpoint ghi thẳng một dòng meta/checkpoints.jsonl với mốc giờ
// cố định: Checkpoints.Append dùng time.Now() nên golden không dùng nó được.
// `at` của bài thẩm duyệt (và createdAt bài học sinh ra) ổn định nhờ đó.
func goldenReviewCheckpoint(t *testing.T, engineDir string, chapter int, at time.Time) {
	t.Helper()
	data, err := json.Marshal(domain.Checkpoint{
		Seq:        int64(chapter),
		Scope:      domain.ChapterScope(chapter),
		Step:       "review",
		Artifact:   store.ReviewScopeChapter.Artifact(chapter),
		OccurredAt: at,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.NewIO(engineDir).AppendLine("meta/checkpoints.jsonl", append(data, '\n')); err != nil {
		t.Fatal(err)
	}
}

// goldenIssue là một mục thẩm duyệt tối thiểu: chỉ trường mà API đọc
// (rule_ref → mục/tiêu đề, severity → mức, description/quote → thẻ).
func goldenIssue(ref, severity, text string) domain.ConsistencyIssue {
	return domain.ConsistencyIssue{
		Type:           "consistency",
		Severity:       severity,
		RequiresChange: domain.RequiresChangeFor(severity, false),
		RuleRef:        ref,
		Description:    text,
		Evidence:       "trích " + text,
		Suggestion:     "sửa " + text,
	}
}

var (
	goldenT1 = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	goldenT2 = time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	goldenT3 = time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
)

// goldenReview lưu một bài thẩm duyệt + checkpoint giờ cố định cho nó.
func goldenReview(t *testing.T, engineDir string, chapter int, ruleAware bool, at time.Time, issues ...domain.ConsistencyIssue) {
	t.Helper()
	st := store.NewStore(engineDir)
	rv := domain.ReviewEntry{
		Chapter: chapter, Scope: "chapter", Verdict: "polish",
		Summary: "Tóm tắt chấm", Issues: issues, RuleAware: ruleAware,
	}
	if rv.Issues == nil {
		rv.Issues = []domain.ConsistencyIssue{}
	}
	if err := st.World.SaveReview(rv); err != nil {
		t.Fatal(err)
	}
	goldenReviewCheckpoint(t, engineDir, chapter, at)
}

// goldenLessonStories là bảng quan sát chung giữa golden của endpoint
// (/api/system/*) và fixture gói trong internal/lessons (violations-*.json):
// [thư mục, tiêu đề, gói, các mục vấp (mục, mức, chương)]. Hai bên phải mô tả
// cùng một bảng — test dưới so handler thật với golden của gói lessons.
var goldenLessonStories = []struct {
	dir, title, pack string
	issues           [][3]any // {ref, level, chapter}
}{
	{"01-tra-xanh", "Trà Xanh Cướp Kịch Bản", "nu-tan-bao-thu", [][3]any{
		{"Master §7", "error", 3}, {"Master §13", "warning", 3}, {"Bộ A §4.3", "warning", 3}, {"Xưng hô", "error", 2}, {"Master §11", "error", 1}, {"Master §15", "warning", 2}}},
	{"02-em-gai", "Em Gái Nuôi Mặc Váy Cưới", "nu-tan-bao-thu", [][3]any{
		{"Master §7", "error", 2}, {"Master §7", "error", 5}, {"Master §13", "warning", 2}, {"Master §8", "error", 2}, {"Master §11", "error", 1}, {"Xưng hô", "error", 2}}},
	{"03-me-ke", "Mẹ Kế Cướp Tiệm May", "nu-tan-bao-thu", [][3]any{
		{"Master §7", "error", 6}, {"Master §13", "warning", 1}, {"Master §15", "warning", 4}, {"Xưng hô", "error", 1}}},
	{"04-thien-kim", "Thiên Kim Thật Trở Về", "nu-tan-bao-thu", [][3]any{
		{"Master §7", "error", 4}, {"Master §13", "warning", 1}}},
	{"05-ban-than", "Bạn Thân Giật Chồng", "nu-tan-bao-thu", [][3]any{
		{"Master §15", "warning", 2}, {"Master §15", "warning", 4}, {"Quy tắc của bạn", "warning", 3}, {"Quy tắc của bạn", "warning", 5}}},
	{"06-chi-dau", "Chị Dâu Bắt May Váy Cưới", "nu-tan-bao-thu", [][3]any{
		{"Bộ A §4.3", "warning", 1}, {"Bộ A §4.3", "warning", 2}, {"Master §11", "error", 3}}},
	{"07-tong-tai", "Tổng Tài Lạnh Lùng Hối Hận", "nguoc-hon-nhan", [][3]any{
		{"Master §4", "error", 1}, {"Master §10", "error", 2}, {"Quy tắc của bạn", "warning", 2}, {"Quy tắc của bạn", "warning", 6}}},
	{"08-nguoi-yeu-cu", "Người Yêu Cũ Quay Lại", "nguoc-hon-nhan", [][3]any{
		{"Master §4", "error", 1}, {"Master §4", "error", 2}, {"Master §10", "error", 1}, {"Master §8", "error", 3}}},
	{"09-lam-tam", "Làm Tổng Tài Giả", "nguoc-hon-nhan", [][3]any{
		{"Master §10", "error", 1}, {"Master §13", "warning", 1}, {"Master §13", "warning", 4}}},
	{"10-em-re", "Em Rể Vào Cửa Sau", "nguoc-hon-nhan", [][3]any{
		{"Master §7", "error", 1}, {"Xưng hô", "warning", 1}}},
	{"11-co-dau", "Cô Dâu Nhỏ Thập Niên Tám Mươi", "nien-dai-van", [][3]any{
		{"Master §6", "error", 2}, {"Master §15", "warning", 2}, {"Master §15", "warning", 4}, {"Quy tắc của bạn", "warning", 2}, {"Quy tắc của bạn", "warning", 4}}},
	{"12-ong-chu", "Ông Chủ Xưởng May", "nien-dai-van", [][3]any{
		{"Master §6", "error", 1}, {"Master §9", "warning", 1}, {"Quy tắc của bạn", "warning", 1}}},
}

// goldenLessonSeeds là các bài học đã tồn tại trong lessons.json của người dùng
// trước đợt quét (bài của người dùng + quyết định giữ/bỏ trước đây), ghi nguyên
// dạng cũ {"count","kind"} — §M không chuyển dữ liệu, store suy ra
// errors/warnings/userEdits lúc đọc. id phải trùng lessons.ID(GroupKey(...)) thì
// Sync mới gộp được đề xuất cùng nhóm — lệch id thì Migrate bỏ thẻ.
func goldenLessonSeeds() []map[string]any {
	created1 := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	created2 := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	created3 := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	decided3 := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	decided4 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	decided5 := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	decided6 := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	legacy := func(count int, kind string, stories int, pack string, packsCount int) map[string]any {
		return map[string]any{"count": count, "kind": kind, "stories": stories, "pack": pack, "packsCount": packsCount}
	}
	return []map[string]any{
		{
			"id": "lesson-user-xung-ho", "origin": "user", "refs": []string{"Xưng hô"},
			"text":     "Khi hai nhân vật cắt đứt quan hệ, đổi cách gọi ngay trong cảnh cắt đứt và ghi lý do; không để chương sau mới đổi.",
			"evidence": legacy(3, "xưng hô", 2, "", 2),
			"scope":    "all", "packId": "nu-tan-bao-thu", "packName": "Nữ tần trọng sinh báo thù, vả mặt",
			"state": "pending", "createdAt": created1,
		},
		{
			"id": lessons.ID(lessons.GroupKey("nu-tan-bao-thu", "Quy tắc của bạn")), "origin": "review", "refs": []string{"Quy tắc của bạn"},
			"text":     "Từ “khẽ” và “dường như” bị lạm dụng ở mọi gói; giữ dưới 2 lần trên 1.000 tiếng.",
			"evidence": legacy(9, "warning", 6, "", 3),
			"scope":    "all", "packId": "nu-tan-bao-thu", "packName": "Nữ tần trọng sinh báo thù, vả mặt",
			"state": "pending", "createdAt": created2,
		},
		{
			"id": lessons.ID(lessons.GroupKey("nu-tan-bao-thu", "Master §12")), "origin": "user", "refs": []string{"Master §12"},
			"text":     "Kết cục phản diện phải được người khác chứng kiến, không chỉ là tự nhận trong phòng kín.",
			"evidence": legacy(2, "kết chương", 2, "Nữ tần trọng sinh báo thù, vả mặt", 1),
			"scope":    "pack", "packId": "nu-tan-bao-thu", "packName": "Nữ tần trọng sinh báo thù, vả mặt",
			"state": "kept", "decidedAt": decided3, "createdAt": created3,
		},
		{
			"id": lessons.ID(lessons.GroupKey("nguoc-hon-nhan", "Master §4")), "origin": "review", "refs": []string{"Master §4"},
			"text":     "Câu đầu tiên luôn có người và có bất công; không mở bằng thời tiết hay căn phòng.",
			"evidence": legacy(2, "error", 2, "Ngược hôn nhân, ly hôn, hối hận muộn", 1),
			"scope":    "pack", "packId": "nguoc-hon-nhan", "packName": "Ngược hôn nhân, ly hôn, hối hận muộn",
			"state": "kept", "decidedAt": decided4, "createdAt": created3,
		},
		{
			"id": lessons.ID(lessons.GroupKey("nguoc-hon-nhan", "Master §10")), "origin": "review", "refs": []string{"Master §10"},
			"text":     "Khoan cho nhân vật được thương trước khi để họ chịu thiệt.",
			"evidence": legacy(3, "error", 3, "Ngược hôn nhân, ly hôn, hối hận muộn", 1),
			"scope":    "pack", "packId": "nguoc-hon-nhan", "packName": "Ngược hôn nhân, ly hôn, hối hận muộn",
			"state": "kept", "decidedAt": decided5, "createdAt": created3,
		},
		{
			"id": lessons.ID(lessons.GroupKey("nu-tan-bao-thu", "Master §15")), "origin": "review", "refs": []string{"Master §15"},
			"text":     "Luôn ngắt đoạn sau 60 tiếng.",
			"evidence": legacy(4, "warning", 2, "Nữ tần trọng sinh báo thù, vả mặt", 1),
			"scope":    "pack", "packId": "nu-tan-bao-thu", "packName": "Nữ tần trọng sinh báo thù, vả mặt",
			"state": "dropped", "decidedAt": decided6, "createdAt": created3,
		},
	}
}

// goldenLessonEdits: văn bản artboard được duyệt qua thao tác "Sửa câu chữ" thật
// của endpoint (ValidateText chạy đúng), không phải chữ cứng trong fixture.
// Khoá "packID|ref" vì §15 có đề xuất ở hai gói.
var goldenLessonEdits = map[string]string{
	"nu-tan-bao-thu|Master §7":       "Không kết chương bằng cảnh nhân vật chính đi ngủ hay về nhà an toàn. Nếu cần cảnh nghỉ, đặt nó ở giữa chương và cắt bằng một tin nhắn, tiếng gõ cửa hoặc một phát hiện.",
	"nu-tan-bao-thu|Master §13":      "Chương đầu hay thiếu thoại (dưới mức của gói). Mở bằng một cuộc đối đáp trước khi kể bối cảnh.",
	"nu-tan-bao-thu|Master §8":       "Vùng giữa truyện cần một nút thắt mới, không chỉ kéo dài cấu hình đầu.",
	"nu-tan-bao-thu|Master §11":      "Kết cục phản diện phải được người khác chứng kiến, không chỉ là tự nhận trong phòng kín.",
	"nu-tan-bao-thu|Bộ A §4.3":       "Trà xanh phải gắn với mạch phản diện, không để nó thành chi tiết trang trí.",
	"nu-tan-bao-thu|Xưng hô":         "Khi hai nhân vật cắt đứt quan hệ, đổi cách gọi ngay trong cảnh cắt đứt và ghi lý do; không để chương sau mới đổi.",
	"nu-tan-bao-thu|Quy tắc của bạn": "Từ “khẽ” và “dường như” bị lạm dụng ở mọi gói; giữ dưới 2 lần trên 1.000 tiếng.",
	"nien-dai-van|Master §6":         "Leo thang phải tăng cược sau mỗi chương, không quay về điểm cũ.",
}

// Giữ/bỏ của golden nằm trong lessons.json seed (quyết định có sẵn trước
// đợt quét) — Decide của endpoint ghi time.Now() nên không thể vào golden.

func TestMockGoldenLessons(t *testing.T) {
	tg := newConfigTestGateway(t)
	for i, s := range goldenLessonStories {
		dir := goldenStoryDir(t, tg.project, s.dir, s.title, s.pack)
		engine := filepath.Join(dir, "output", "novel")
		st := store.NewStore(engine)
		if err := st.Characters.Save([]domain.Character{{Name: "Nhân vật " + s.dir, Aliases: []string{"NV " + s.dir}}}); err != nil {
			t.Fatal(err)
		}
		// Gom issues theo chương: một bài thẩm duyệt cho mỗi chương có mục.
		byChapter := map[int][]domain.ConsistencyIssue{}
		chapters := []int{}
		for _, row := range s.issues {
			ref, level, ch := row[0].(string), row[1].(string), row[2].(int)
			issue := goldenIssue(ref, level, fmt.Sprintf("%s vấp %s", s.title, ref))
			// Mục xưng hô/quy tắc riêng đi kèm tên mục qua rule_text (issueRef rơi
			// về rule_text khi ref không có trong gói); mục không có tên bị bỏ
			// khỏi bảng quan sát.
			switch ref {
			case "Xưng hô":
				issue.RuleText = "Bảng xưng hô của truyện"
			case "Quy tắc của bạn":
				issue.RuleText = "Giới hạn từ lướt do người dùng đặt"
			}
			if _, ok := byChapter[ch]; !ok {
				chapters = append(chapters, ch)
			}
			byChapter[ch] = append(byChapter[ch], issue)
		}
		slices.Sort(chapters)
		for j, ch := range chapters {
			goldenReview(t, engine, ch, false, goldenT1.Add(time.Duration(i*len(chapters)+j)*time.Hour), byChapter[ch]...)
		}
	}
	// lessons.json đã có sẵn: bài của người dùng + một bài đã sửa câu chữ.
	io := store.NewIO(filepath.Join(tg.home, ".ainovel", "rules"))
	if err := io.WriteJSON("lessons.json", map[string]any{"schema": 2, "lessons": goldenLessonSeeds()}); err != nil {
		t.Fatal(err)
	}
	admin := tg.login("admin", "admin-pass")

	// Quét lần đầu để lấy id thật của các đề xuất, rồi quyết/sửa qua endpoint.
	list := lessonResponse(t, tg, admin, "pending")
	byKey := map[string]string{}
	for _, l := range list {
		byKey[l.PackID+"|"+l.Refs[0]] = l.ID
	}
	for key, text := range goldenLessonEdits {
		id, ok := byKey[key]
		if !ok {
			t.Fatalf("không có bài học chờ duyệt cho %s", key)
		}
		payload, _ := json.Marshal(map[string]string{"action": "edit", "text": text})
		if res := tg.do("POST", "/api/system/lessons/"+id, admin, string(payload)); res.Code != 200 {
			t.Fatalf("edit %s: %d %s", key, res.Code, res.Body.String())
		}
	}

	for _, state := range []string{"pending", "kept", "dropped"} {
		res := tg.do("GET", "/api/system/lessons?state="+state, admin, "")
		if res.Code != 200 {
			t.Fatalf("lessons %s: %d %s", state, res.Code, res.Body.String())
		}
		assertMockGolden(t, "lessons-"+state+".json", res.Body.Bytes())
	}
	// violations-*.json do internal/lessons ghi; handler phải trả đúng các byte đó
	// (cùng bảng quan sát → chỉ sửa bảng một bên sẽ lộ lệch ngay).
	for _, pack := range []string{"nu-tan-bao-thu", "nguoc-hon-nhan", "nien-dai-van"} {
		res := tg.do("GET", "/api/system/violations?pack="+pack, admin, "")
		if res.Code != 200 {
			t.Fatalf("violations %s: %d %s", pack, res.Code, res.Body.String())
		}
		assertGoldenPinned(t, "violations-"+pack+".json", res.Body.Bytes())
	}
}

// ─── Sổ cái (ledger) ────────────────────────────────────────────────────────

// mockLedgerSeed mô tả dùng chung với bảng `seed` cũ của mock/projbrain.ts:
// [thư mục, tiêu đề, 10 trục]. Handler trả stories mới nhất trước (đảo tên thư
// mục), nên golden khác thứ tự seed cũ — mock phát lại nguyên thứ tự đó.
var mockLedgerSeed = []struct {
	dir, title string
	values     ledger.Values
}{
	{"01-tra-xanh-cuop-kich-ban", "Trà Xanh Cướp Kịch Bản Của Tôi, Tôi Lên Show Thực Tế Đọc Bản Gốc",
		ledger.Values{"A showbiz", "Đoàn phim", "Biên kịch", "Bạn diễn", "Mẹ già", "Bản ghi âm gốc", "Livestream", "Mỏ hỗn sắc", "Câu thoại bất công", "Sự nghiệp riêng"}},
	{"me-ke-tiem-may", "Mẹ Kế Cướp Tiệm May, Tôi Trọng Sinh Lấy Lại Từng Đường Kim",
		ledger.Values{"A trọng sinh", "Khu chợ", "Thợ may", "Mẹ kế", "Món đồ của mẹ", "Sổ sách viết tay", "Chợ phiên", "Điềm tĩnh lạnh", "Khoảnh khắc chết", "Rời đi"}},
	{"chong-cu-quan-pho", "Chồng Cũ Hối Hận Khi Tôi Mở Quán Phở Đầu Ngõ",
		ledger.Values{"B", "Phố cổ", "Đầu bếp", "Chồng", "Con nhỏ", "Ghi âm", "Đám giỗ", "Ấm, khóc được", "Một quyết định", "Tình cảm mới"}},
	{"ganh-xoi", "Năm Tám Mươi, Tôi Nuôi Cả Nhà Bằng Gánh Xôi",
		ledger.Values{"C-Nữ", "Thập niên 80", "Bán xôi", "Họ hàng", "Bà nội", "Sổ sách viết tay", "Chợ phiên", "Ấm, khóc được", "Một quyết định", "Đoàn tụ"}},
	{"thien-kim", "Thiên Kim Thật Trở Về, Cả Họ Quỳ Trước Cổng",
		ledger.Values{"A thiên kim", "Hào môn", "Y tá", "Em nuôi", "Mẹ già", "Bản xét nghiệm", "Đám giỗ", "Mỏ hỗn sắc", "Khoảnh khắc tỉnh lại", "Đoàn tụ"}},
	{"ban-than", "Bạn Thân Giật Chồng, Tôi Lật Sổ Đỏ Giữa Lễ Đính Hôn",
		ledger.Values{"B", "Làng chài", "Kế toán", "Bạn thân", "Con nhỏ", "Sổ đỏ", "Lễ đính hôn", "Điềm tĩnh lạnh", "Khoảnh khắc chết", "Rời đi"}},
	{"em-gai-nuoi", "Em Gái Nuôi Mặc Váy Cưới Của Tôi Lên Livestream",
		ledger.Values{"A trọng sinh", "Công ty", "Thợ may", "Em nuôi", "Con vật nuôi", "Bản nháp có dấu thời gian", "Livestream", "Hài", "Khoảnh khắc tỉnh lại", "Sự nghiệp riêng"}},
	{"dam-gio", "Đám Giỗ Cả Họ, Tôi Đọc Di Chúc Của Bà",
		ledger.Values{"C-Nữ", "Thập niên 80", "Thợ mộc", "Họ hàng", "Bà nội", "Di chúc", "Đám giỗ", "Ấm, khóc được", "Câu thoại bất công", "Đoàn tụ"}},
	{"dong-nghiep", "Đồng Nghiệp Cướp Công, Tôi Trình Bày Lại Trước Đại Hội Cổ Đông",
		ledger.Values{"A trọng sinh", "Công ty", "Kế toán", "Đồng nghiệp", "Mẹ già", "Email gốc", "Đại hội cổ đông", "Điềm tĩnh lạnh", "Khoảnh khắc chết", "Sự nghiệp riêng"}},
	{"me-chong", "Mẹ Chồng Đuổi Tôi Đi, Tôi Thành Chủ Chuỗi Tiệm Bánh",
		ledger.Values{"B", "Nhà hàng", "Đầu bếp", "Mẹ chồng", "Con nhỏ", "Ghi âm", "Phiên toà", "Ấm, khóc được", "Một quyết định", "Sự nghiệp riêng"}},
	{"hidden-story", "Truyện riêng không được xem",
		ledger.Values{"Bí mật", "Bí mật", "Bí mật", "Bí mật", "Bí mật", "Bí mật", "Bí mật", "Bí mật", "Bí mật", "Bí mật"}},
}

var mockLedgerCandidateValues = ledger.Values{
	"A trọng sinh", "Khu chợ", "Thợ may", "Mẹ kế", "Mẹ già", "Sổ sách viết tay", "Lễ đính hôn", "Mỏ hỗn sắc", "Khoảnh khắc tỉnh lại", "Sự nghiệp riêng",
}

// mockLedgerBrief là brief của ứng viên: trùng brief mà mock dùng để so sổ cái,
// nên handler tự loại ứng viên khỏi tập truyện trước (candidate.Brief == brief).
var mockLedgerBrief = "Thợ May Trọng Sinh, Mẹ Kế Bắt Tôi May Váy Cưới Cho Em Gái"

// mockLedgerAdded là ngày thêm truyện (story.json › createdAt), dòng phụ thứ hai
// của bảng sổ cái ProjBrain. Cố định để golden không phụ thuộc đồng hồ máy; giảm
// dần theo thứ tự handler trả (mới nhất trước) như artboard 24/10 → 10/10.
var mockLedgerAdded = map[string]string{
	"thien-kim":                 "2026-10-24",
	"me-ke-tiem-may":            "2026-10-22",
	"me-chong":                  "2026-10-21",
	"hidden-story":              "2026-10-20",
	"ganh-xoi":                  "2026-10-19",
	"em-gai-nuoi":               "2026-10-17",
	"dong-nghiep":               "2026-10-16",
	"dam-gio":                   "2026-10-13",
	"chong-cu-quan-pho":         "2026-10-12",
	"ban-than":                  "2026-10-10",
	"01-tra-xanh-cuop-kich-ban": "2026-09-28",
}

func TestMockGoldenLedger(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	admin := tg.login("admin", "admin-pass")

	// Mọi truyện có sổ cái đều đã có dàn ý ngoại trừ ứng viên; hidden-story là
	// truyện riêng mà reader "limited" không đọc được.
	for _, s := range mockLedgerSeed {
		dir := goldenStoryDir(t, tg.project, s.dir, s.title, "")
		if raw, ok := mockLedgerAdded[s.dir]; ok {
			added, err := time.Parse(time.DateOnly, raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := library.WriteStoryFile(dir, library.StoryFile{Title: s.title, CreatedAt: &added}); err != nil {
				t.Fatal(err)
			}
		}
		out := filepath.Join(dir, "output", "novel")
		if err := ledger.Save(out, ledger.Record{Values: s.values, Brief: "Brief " + s.dir, Result: ledger.CheckValues(s.values, nil)}); err != nil {
			t.Fatal(err)
		}
		if err := store.NewStore(out).Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Mở đầu"}}); err != nil {
			t.Fatal(err)
		}
	}
	{
		out := filepath.Join(goldenStoryDir(t, tg.project, "02-chua-co-ten", "Chưa có tên", ""), "output", "novel")
		// Ứng viên: sổ cái của chính nó chấm so với mọi truyện khác (handler loại
		// nó khỏi tập trước vì brief trùng), chưa có dàn ý.
		prior := make([]ledger.Story, 0, len(mockLedgerSeed))
		for _, s := range mockLedgerSeed {
			prior = append(prior, ledger.Story{ID: "proj/" + s.dir, Title: s.title, Values: s.values, Opening: s.values[8]})
		}
		if err := ledger.Save(out, ledger.Record{Values: mockLedgerCandidateValues, Brief: mockLedgerBrief, Result: ledger.CheckValues(mockLedgerCandidateValues, prior)}); err != nil {
			t.Fatal(err)
		}
	}

	res := tg.do("GET", "/api/projects/proj/ledger", admin, "")
	if res.Code != 200 {
		t.Fatalf("GET ledger: %d %s", res.Code, res.Body.String())
	}
	assertMockGolden(t, "ledger.json", res.Body.Bytes())

	// POST check của handler phải khớp đúng golden ghi bởi internal/ledger
	// (cùng bảng truyện + cùng giá trị ứng viên) — đây là kiểm tra nhất quán
	// giữa hai fixture thay vì ghi đè file của package kia.
	v := mockLedgerCandidateValues
	ledgerStub(t, tg, &v)
	res = tg.do("POST", "/api/projects/proj/ledger/check", admin, `{"brief":"`+mockLedgerBrief+`"}`)
	if res.Code != 200 {
		t.Fatalf("check: %d %s", res.Code, res.Body.String())
	}
	assertGoldenPinned(t, "ledger-check.json", res.Body.Bytes())

	// suggest cần đề xuất không còn trục bắt sửa: stub trả về bộ trục đã sửa
	// (handler tự tính checks của nó — nên trùng đúng ledger-check-fixed.json).
	v = ledger.Values{
		"A trọng sinh", "Khu chợ", "Thợ may", "Chủ tiệm", "Mẹ già", "Sổ sách viết tay", "Lễ đính hôn", "Mỏ hỗn sắc", "Khoảnh khắc tỉnh lại", "Sự nghiệp riêng",
	}
	res = tg.do("POST", "/api/projects/proj/ledger/suggest", admin, `{"brief":"Yêu cầu"}`)
	if res.Code != 200 {
		t.Fatalf("suggest: %d %s", res.Code, res.Body.String())
	}
	assertMockGolden(t, "ledger-suggest.json", res.Body.Bytes())
}

// ─── Vũ trụ chung (universe) ────────────────────────────────────────────────

// goldenUniverseStorySeed: truyện của dự án kenh — [thư mục, tiêu đề, nhân vật
// {tên, vai}, địa điểm của chương 1 {tên, mô tả}]. Thư mục NN- định thứ tự đọc
// của handler (viewableProjectStories đi theo tên thư mục).
var goldenUniverseStorySeed = []struct {
	dir, title string
	chars      [][2]string
	place      [2]string
}{
	{"01-tra-xanh-cuop-kich-ban", "Trà Xanh Cướp Kịch Bản", [][2]string{{"Bà Tư bán xôi", "Khách quen của chợ"}, {"Mạc Thanh", "Nhà sản xuất trong show"}}, [2]string{"Đài truyền hình Vân Châu", "Nơi quay show thực tế"}},
	{"me-ke-tiem-may", "Mẹ Kế Cướp Tiệm May", [][2]string{{"Mẹ kế", "Chủ tiệm may cũ"}}, [2]string{"Chợ Vân Châu", "Chợ phiên đầu phố"}},
	{"ban-than", "Bạn Thân Giật Chồng", [][2]string{{"Luật sư Hàn", "Luật sư của bạn thân"}, {"Bạn thân", "Nhân vật phụ"}}, [2]string{"", ""}},
	{"ganh-xoi", "Gánh Xôi Năm Tám Mươi", [][2]string{{"Bà Tư bán xôi", "Bà Tư bán xôi đầu ngõ"}}, [2]string{"Chợ Vân Châu", "Sân khấu cao trào (chợ phiên)"}},
	{"dam-gio", "Đám Giỗ Cả Họ", [][2]string{{"Bà Tư bán xôi", "Hàng xóm, nhân chứng ở đám giỗ"}}, [2]string{"", ""}},
	{"dong-nghiep", "Đồng Nghiệp Cướp Công", [][2]string{{"Cô lao công Lụa", "Người giữ bản email gốc"}, {"Đồng nghiệp", "Nhân vật phụ"}}, [2]string{"", ""}},
	{"em-gai-nuoi", "Em Gái Nuôi Mặc Váy Cưới", [][2]string{{"Em nuôi", "Em gái nuôi"}}, [2]string{"Đài truyền hình Vân Châu", "Trường quay livestream"}},
	{"me-chong", "Mẹ Chồng Đuổi Tôi Đi", [][2]string{{"Luật sư Hàn", "Luật sư phía nữ chính ở phiên toà"}, {"Mẹ chồng", "Nhân vật phụ"}}, [2]string{"", ""}},
	{"thien-kim", "Thiên Kim Thật Trở Về", [][2]string{{"Cô lao công Lụa", "Người lao công biết bí mật"}, {"Em nuôi", "Em nuôi nhà thiên kim"}}, [2]string{"", ""}},
	{"hidden-story", "Truyện riêng không được xem", [][2]string{{"Nhân vật bí mật", "Không được tiết lộ"}}, [2]string{"Phòng bí mật", "Không được tiết lộ"}},
}

// TestMockGoldenUniverse ghi universe.json (view đầy đủ của admin) và
// universe-verdicts.json (mục Master §19 theo từng truyện, kèm cờ đã chấm) —
// mock lọc theo quyền xem từ hai file này.
func TestMockGoldenUniverse(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	// Dự án riêng có tên vũ trụ; một truyện ẩn với mọi tài khoản fixture.
	kenh := filepath.Join(tg.root, "kenh")
	if err := os.MkdirAll(kenh, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kenh, "project.json"), []byte(`{"name":"Kênh","universeName":"Thành phố Vân Châu"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dirs := map[string]string{}
	for i, s := range goldenUniverseStorySeed {
		dir := goldenStoryDir(t, kenh, s.dir, s.title, "")
		dirs[s.dir] = dir
		engine := filepath.Join(dir, "output", "novel")
		st := store.NewStore(engine)
		chars := []domain.Character{}
		for _, c := range s.chars {
			chars = append(chars, domain.Character{Name: c[0], Role: c[1]})
		}
		if err := st.Characters.Save(chars); err != nil {
			t.Fatal(err)
		}
		if err := st.Progress.Save(&domain.Progress{CompletedChapters: []int{1}}); err != nil {
			t.Fatal(err)
		}
		if s.place[0] != "" {
			rec := domain.ChapterRecord{
				Version: 1, Chapter: 1, Revision: 1, Origin: domain.ChapterOriginGenerated,
				Content: "Nội dung chương 1.", AcceptedAt: goldenT1,
				Facts: domain.ChapterFacts{Places: []domain.FactEntity{{Name: s.place[0], Desc: s.place[1]}}},
			}
			rec.ContentSHA256 = domain.ChapterContentSHA256(rec.Content)
			if err := st.ChapterRecords.Save(rec); err != nil {
				t.Fatal(err)
			}
		}
		_ = i
	}
	// Vòng chấm có `rule` (s19-recap): tra-xanh và dam-gio còn mục mở, ganh-xoi
	// và me-chong có mục đã "Giữ nguyên", ban-than đã chấm mà sạch.
	awareReviews := []struct {
		dir     string
		chapter int
		item    domain.ConsistencyIssue
		at      time.Time
	}{
		{"01-tra-xanh-cuop-kich-ban", 6, domain.ConsistencyIssue{
			Type: "consistency", Severity: "warning", RuleRef: "Master §19", Rule: "s19-recap",
			Description: "Đoạn mở chương nhắc lại 4 câu về buổi ghi hình cũ; Master §19 cho tối đa 2 câu, lồng vào hành động.",
			Evidence:    "Tôi nhớ lại buổi ghi hình hôm ấy, cái ống kính, ánh đèn, tiếng đạo diễn hô, và cả câu nói của Tri Hạ.",
			Suggestion:  "Giữ một câu hồi tưởng, chuyển phần còn lại thành hành động đang diễn ra."}, goldenT1},
		{"ban-than", 2, domain.ConsistencyIssue{}, goldenT1},
		{"dam-gio", 2, domain.ConsistencyIssue{
			Type: "consistency", Severity: "warning", RuleRef: "Master §19", Rule: "s19-recap",
			Description: "“Bà Tư bán xôi” ở chương 2 của “Đám Giỗ Cả Họ, Tôi Đọc Di Chúc Của Bà” nhắc lại 4 câu về truyện cũ. Master §19 cho tối đa 2 câu, lồng vào hành động.",
			Evidence:    "Bà Tư lại nhắc chuyện gánh xôi buổi sáng như mọi lần.",
			Suggestion:  "Lồng hồi tưởng vào hành động dọn cỗ."}, goldenT2},
		{"ganh-xoi", 3, domain.ConsistencyIssue{
			Type: "consistency", Severity: "warning", RuleRef: "Master §19", Rule: "s19-recap",
			Description: "Đoạn mở chương nhắc lại 4 câu về gánh xôi buổi sáng; Master §19 cho tối đa 2 câu, lồng vào hành động.",
			Evidence:    "Gánh xôi của bà Tư lại đi qua đầu ngõ.",
			Suggestion:  "Rút ngắn hồi tưởng."}, goldenT1},
		{"me-chong", 5, domain.ConsistencyIssue{
			Type: "consistency", Severity: "warning", RuleRef: "Master §19", Rule: "s19-recap",
			Description: "“Mẹ chồng đuổi tôi đi” ở chương 5 nhắc lại 4 câu về phiên toà cũ; Master §19 cho tối đa 2 câu, lồng vào hành động.",
			Evidence:    "Phiên toà hôm ấy vẫn vang trong đầu tôi.",
			Suggestion:  "Chuyển hồi tưởng thành hành động ở phiên toà mới."}, goldenT3},
	}
	for _, rv := range awareReviews {
		engine := filepath.Join(dirs[rv.dir], "output", "novel")
		var issues []domain.ConsistencyIssue
		if rv.item.Description != "" {
			issues = []domain.ConsistencyIssue{rv.item}
		}
		goldenReview(t, engine, rv.chapter, true, rv.at, issues...)
	}
	// Quyết định "Giữ nguyên" của ganh-xoi và me-chong: khoá là itemId ổn định
	// mà handler sinh (băm ref+chương+trích), do đó tính bằng chính hàm của
	// handler chứ không ghi tay.
	kept := []struct{ dir, reason, by, at string }{
		{"ganh-xoi", "Người nghe mới cần biết vì sao bà Tư có mặt ở đám giỗ", "Dat", "2026-10-24T12:00:00Z"},
		{"me-chong", "Hồi tưởng có chủ ý, là manh mối của chương", "Dat", "2026-10-22T12:00:00Z"},
	}
	for _, k := range kept {
		var rv *struct {
			dir     string
			chapter int
			item    domain.ConsistencyIssue
			at      time.Time
		}
		for i := range awareReviews {
			if awareReviews[i].dir == k.dir {
				rv = &awareReviews[i]
			}
		}
		if rv == nil {
			t.Fatalf("không có bài chấm aware cho %s", k.dir)
		}
		engine := filepath.Join(dirs[k.dir], "output", "novel")
		st := store.NewStore(engine)
		used := map[string]int{}
		itemID := domain.ReviewItemID(rv.chapter, used, "Master §19", rv.item.Evidence)
		if err := st.World.SaveReviewDecision(itemID, store.ReviewDecision{
			State: "kept", Reason: k.reason, By: k.by, At: k.at,
			Ref: "Master §19", Text: rv.item.Description, Quote: rv.item.Evidence, Chapter: rv.chapter,
		}); err != nil {
			t.Fatal(err)
		}
	}

	admin := tg.login("admin", "admin-pass")
	res := tg.do("GET", "/api/projects/kenh/universe", admin, "")
	if res.Code != 200 {
		t.Fatalf("universe: %d %s", res.Code, res.Body.String())
	}
	assertMockGolden(t, "universe.json", res.Body.Bytes())

	// verdicts theo từng truyện dùng chung: mock gộp lại sau khi lọc quyền xem.
	lib, err := library.Open(tg.root)
	if err != nil {
		t.Fatal(err)
	}
	brains, err := tg.g.projectBrains(context.Background(), library.FileRepository(lib), webauth.User{Username: "admin", Role: webauth.RoleAdmin}, "kenh")
	if err != nil {
		t.Fatal(err)
	}
	type storyVerdicts struct {
		Story     string              `json:"story"`
		Evaluated bool                `json:"evaluated"`
		Checks    []universeCheck     `json:"checks"`
		Kept      []universeKeptCheck `json:"kept"`
	}
	shared := universeSharedStories(t, brains)
	verdicts := []storyVerdicts{}
	for _, sb := range brains {
		if !shared[sb.ref.ID] {
			continue
		}
		checks, kept, evaluated, err := storyUniverseChecks(sb)
		if err != nil {
			t.Fatal(err)
		}
		verdicts = append(verdicts, storyVerdicts{Story: sb.ref.ID, Evaluated: evaluated, Checks: checks, Kept: kept})
	}
	data, err := json.Marshal(map[string]any{"stories": verdicts})
	if err != nil {
		t.Fatal(err)
	}
	assertMockGolden(t, "universe-verdicts.json", data)
}

// universeSharedStories là tập `inUniverse` của serveProjectUniverse: truyện có
// ít nhất một thực thể dùng chung (entity ≥ 2 truyện sau khi gộp).
func universeSharedStories(t *testing.T, brains []projectStoryBrain) map[string]bool {
	t.Helper()
	shared := map[string]bool{}
	for _, entity := range mergeProjectEntities(brains, "char", "place") {
		if len(entity.Stories) < 2 {
			continue
		}
		for _, ref := range entity.Stories {
			shared[ref.ID] = true
		}
	}
	return shared
}

// storyUniverseChecks gom mục s19-recap của một truyện: cùng bộ lọc và cách gắn
// quyết định của universeChecks, nhưng theo truyện để mock phát lại khi lọc.
func storyUniverseChecks(sb projectStoryBrain) (checks []universeCheck, kept []universeKeptCheck, evaluated bool, err error) {
	checks = []universeCheck{}
	kept = []universeKeptCheck{}
	cards, err := newReviewReader(sb.dir, sb.reader).cards()
	if err != nil {
		return nil, nil, false, err
	}
	seen := map[string]bool{}
	for _, card := range cards {
		dto := card.View
		add := func(items []reviewItemAPI) {
			for _, item := range items {
				if item.Ref != universeCheckRef || item.Rule != universeCheckRule {
					continue
				}
				if seen[item.ID] {
					continue
				}
				seen[item.ID] = true
				row := universeCheck{Ref: item.Ref, Story: sb.ref.ID, Ch: dto.Chapter, Text: item.Text, QuoteID: item.ID}
				if reviewItemKept(item) {
					kept = append(kept, universeKeptCheck{universeCheck: row, Reason: item.Decision.Reason, DecidedBy: item.Decision.By, DecidedAt: item.Decision.At})
					continue
				}
				checks = append(checks, row)
			}
		}
		if dto.RuleAware {
			evaluated = true
			add(dto.Items)
		}
		for i := range dto.Previous {
			if !dto.Previous[i].RuleAware {
				continue
			}
			evaluated = true
			add(dto.Previous[i].Items)
		}
	}
	return checks, kept, evaluated, nil
}

// ─── Chất lượng truyện (quality) ────────────────────────────────────────────

// goldenQualityStory dựng truyện có n chương hoàn thành trong dàn ý tổng
// `total`, các bài thẩm duyệt và log viết lại đã cho. Trả về dir truyện.
func goldenQualityStory(t *testing.T, tg *configTestGateway, name, title string, completed, total int, pending []int, rewrites []domain.RewriteEntry, reviews map[int][]domain.ConsistencyIssue, reviewedTwice map[int][]domain.ConsistencyIssue) string {
	t.Helper()
	dir := goldenStoryDir(t, tg.project, name, title, "nu-tan-bao-thu")
	engine := filepath.Join(dir, "output", "novel")
	st := store.NewStore(engine)
	entries := []domain.OutlineEntry{}
	for ch := 1; ch <= total; ch++ {
		entries = append(entries, domain.OutlineEntry{Chapter: ch, Title: fmt.Sprintf("Chương %d", ch)})
	}
	if err := st.Outline.SaveOutline(entries); err != nil {
		t.Fatal(err)
	}
	done := []int{}
	for ch := 1; ch <= completed; ch++ {
		done = append(done, ch)
	}
	progress := &domain.Progress{CompletedChapters: done, TotalChapters: total, PendingRewrites: pending}
	if len(pending) > 0 {
		progress.InProgressChapter = pending[0]
	}
	if err := st.Progress.Save(progress); err != nil {
		t.Fatal(err)
	}
	// Log viết lại ghi thẳng để giữ mốc giờ cố định (Progress ghi time.Now()).
	for _, entry := range rewrites {
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.NewIO(engine).AppendLine("meta/rewrites.jsonl", append(data, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	chapters := []int{}
	for ch := range reviews {
		chapters = append(chapters, ch)
	}
	slices.Sort(chapters)
	at := goldenT1
	for _, ch := range chapters {
		// Hai lần ghi để có vòng lưu trữ (mục fixed) khi reviewedTwice có mục.
		if prev, ok := reviewedTwice[ch]; ok {
			goldenReview(t, engine, ch, false, at, prev...)
			at = at.Add(time.Hour)
		}
		goldenReview(t, engine, ch, false, at, reviews[ch]...)
		at = at.Add(time.Hour)
	}
	return dir
}

// goldenStory tạo library.Story cho một thư mục fixture đã dựng — storyReq gọi
// serveStoryRead trực tiếp, không đi qua lib.ResolveStory.
func goldenStory(t *testing.T, tg *configTestGateway, dir string) library.Story {
	t.Helper()
	storyDir := filepath.Join(tg.project, "kich_ban", dir)
	if _, err := os.Stat(storyDir); err != nil {
		t.Fatalf("story dir %s: %v", dir, err)
	}
	return library.Story{Project: "proj", Story: dir, ID: "proj/" + dir, Dir: storyDir, ProjectDir: tg.project, Root: tg.root}
}

func TestMockGoldenQuality(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()

	// Truyện chính của mock: 3 chương đã viết, 4–8 đang trong hàng viết lại,
	// chương 4 đang viết; bài chấm ở chương 3/5/6, chương 2 từng viết lại xong.
	rewrites := []domain.RewriteEntry{
		{Chapter: 2, At: goldenT1.Format("2006-01-02T15:04:05Z07:00"), Cause: "review"},
		{Chapter: 4, At: goldenT2.Format("2006-01-02T15:04:05Z07:00"), Cause: "review"},
		{Chapter: 5, At: goldenT2.Format("2006-01-02T15:04:05Z07:00"), Cause: "review"},
		{Chapter: 6, At: goldenT2.Format("2006-01-02T15:04:05Z07:00"), Cause: "review"},
		{Chapter: 7, At: goldenT2.Format("2006-01-02T15:04:05Z07:00"), Cause: "review"},
		{Chapter: 8, At: goldenT2.Format("2006-01-02T15:04:05Z07:00"), Cause: "review"},
	}
	goldenQualityStory(t, tg, "01-tra-xanh-cuop-kich-ban", "Trà Xanh Cướp Kịch Bản", 3, 8, []int{4, 5, 6, 7, 8}, rewrites,
		map[int][]domain.ConsistencyIssue{
			3: {goldenIssue("Master §7", "error", "Chương kết ở trạng thái an toàn"),
				goldenIssue("Master §13", "warning", "Thoại dưới chuẩn của gói"),
				goldenIssue("Bộ A §4.3", "warning", "Trà xanh chưa gắn phản diện")},
			5: {goldenIssue("Master §15", "error", "Đoạn văn dài quá 90 tiếng"),
				goldenIssue("Master §2", "warning", "Nhịp mạch gấp")},
			6: {goldenIssue("Master §19", "warning", "Nhắc chuyện cũ quá dài")},
		},
		map[int][]domain.ConsistencyIssue{
			3: {goldenIssue("Master §7", "error", "Vòng 1: kết an toàn")},
		})

	st := goldenStory(t, tg, "01-tra-xanh-cuop-kich-ban")

	// Truyện đã sạch sau hai vòng chấm (giang-ho-tri của mock): mục vòng 1 sang
	// trạng thái fixed.
	goldenQualityStory(t, tg, "04-giang-ho-tri", "Giang Hồ Trí", 3, 8, nil, nil,
		map[int][]domain.ConsistencyIssue{
			3: {goldenIssue("Master §13", "warning", "Thoại còn ít"), goldenIssue("Bộ A §4.3", "warning", "Trà xanh chưa gắn")},
		},
		map[int][]domain.ConsistencyIssue{
			3: {goldenIssue("Master §7", "error", "Vòng 1: kết an toàn")},
		})
	st = goldenStory(t, tg, "04-giang-ho-tri")
	res := storyReq(t, tg.g, "GET", "/x/quality", adminUser(), st, "/quality", "")
	if res.Code != 200 {
		t.Fatalf("quality clean: %d %s", res.Code, res.Body.String())
	}
	assertMockGolden(t, "quality-clean.json", res.Body.Bytes())

	// Truyện còn bắt sửa ở chương 9 (legacy giang-ho-tri): cửa duyệt đóng.
	goldenQualityStory(t, tg, "99-giang-ho-tri", "Giang Hồ Trí", 9, 20, nil, nil,
		map[int][]domain.ConsistencyIssue{
			9: {goldenIssue("Master §7", "error", "Kết chương treo chưa đạt"),
				goldenIssue("Master §2", "warning", "Móc mở đầu nhạt")},
		}, nil)
	st = goldenStory(t, tg, "99-giang-ho-tri")
	res = storyReq(t, tg.g, "GET", "/x/quality", adminUser(), st, "/quality", "")
	if res.Code != 200 {
		t.Fatalf("quality blocking: %d %s", res.Code, res.Body.String())
	}
	assertMockGolden(t, "quality-blocking.json", res.Body.Bytes())

	// Truyện dài của màn Chất lượng: 40 chương đã viết, 35–40 trong hàng viết
	// lại; chương chẵn có hai vòng chấm (mục vòng 1 → fixed).
	reviews40 := map[int][]domain.ConsistencyIssue{}
	twice40 := map[int][]domain.ConsistencyIssue{}
	refs := []string{"Master §2", "Master §4", "Master §5", "Master §6", "Master §7", "Master §12", "Master §13", "Master §15", "Master §16"}
	for ch := 1; ch <= 40; ch++ {
		var issues []domain.ConsistencyIssue
		for i, ref := range refs {
			if (ch*7+i*11)%4 == 0 {
				level := "warning"
				if (ch+i)%5 == 0 {
					level = "error"
				}
				issues = append(issues, goldenIssue(ref, level, fmt.Sprintf("Chương %d vấp %s", ch, ref)))
			}
		}
		if len(issues) > 0 {
			reviews40[ch] = issues
			if ch%2 == 0 {
				twice40[ch] = []domain.ConsistencyIssue{goldenIssue(refs[ch%len(refs)], "warning", fmt.Sprintf("Vòng 1 chương %d", ch))}
			}
		}
	}
	pending40 := []int{35, 36, 37, 38, 39, 40}
	rewrites40 := []domain.RewriteEntry{}
	for _, ch := range pending40 {
		rewrites40 = append(rewrites40, domain.RewriteEntry{Chapter: ch, At: goldenT3.Format("2006-01-02T15:04:05Z07:00"), Cause: "review"})
	}
	goldenQualityStory(t, tg, "98-chat-luong-40", "Truyện Dài Bốn Mươi Chương", 40, 40, pending40, rewrites40, reviews40, twice40)
	st = goldenStory(t, tg, "98-chat-luong-40")
	res = storyReq(t, tg.g, "GET", "/x/quality", adminUser(), st, "/quality", "")
	if res.Code != 200 {
		t.Fatalf("quality 40: %d %s", res.Code, res.Body.String())
	}
	assertMockGolden(t, "quality-40.json", res.Body.Bytes())

	// Truyện chưa chấm chương nào: không có thẻ/ô, chương đã viết là Đã duyệt.
	goldenQualityStory(t, tg, "97-no-review", "Chưa Chấm", 2, 8, nil, nil, nil, nil)
	st = goldenStory(t, tg, "97-no-review")
	res = storyReq(t, tg.g, "GET", "/x/quality", adminUser(), st, "/quality", "")
	if res.Code != 200 {
		t.Fatalf("quality empty: %d %s", res.Code, res.Body.String())
	}
	assertMockGolden(t, "quality-empty.json", res.Body.Bytes())

	// Một mục đã "Giữ nguyên" ở truyện chính: golden giữ trạng thái kept và
	// keptReason, chứng minh mock không cần phân chia lại.
	st = goldenStory(t, tg, "01-tra-xanh-cuop-kich-ban")
	used := map[string]int{}
	keepID := domain.ReviewItemID(5, used, "Master §2", "trích Nhịp mạch gấp")
	if err := store.NewStore(filepath.Join(st.Dir, "output", "novel")).World.SaveReviewDecision(keepID, store.ReviewDecision{
		State: "kept", Reason: "Nhịp cố ý gấp để giữ nhịp mạch", By: "Dat", At: goldenT3.Format("2006-01-02T15:04:05Z07:00"),
		Ref: "Master §2", Text: "Nhịp mạch gấp", Chapter: 5,
	}); err != nil {
		t.Fatal(err)
	}
	res = storyReq(t, tg.g, "GET", "/x/quality", adminUser(), st, "/quality", "")
	if res.Code != 200 {
		t.Fatalf("quality kept: %d %s", res.Code, res.Body.String())
	}
	assertMockGolden(t, "quality.json", res.Body.Bytes())
}

// ─── Vi phạm quy tắc máy (rules/violations) ─────────────────────────────────

func TestMockGoldenRuleViolations(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	dir := goldenStoryDir(t, tg.project, "01-story", "Truyện Thử Nghiệm", "")
	engine := filepath.Join(dir, "output", "novel")
	io := store.NewIO(engine)
	writeViolations := func(ch int, at time.Time, vs []rules.Violation) {
		t.Helper()
		data, err := json.Marshal(store.ChapterViolations{Chapter: ch, At: at.Format("2006-01-02T15:04:05Z07:00"), Violations: vs})
		if err != nil {
			t.Fatal(err)
		}
		if err := io.AppendLine("meta/rule_violations.jsonl", append(data, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	writeViolations(1, goldenT1, []rules.Violation{{Rule: "forbidden_phrases", Target: "bỗng nhiên", Actual: 1, Severity: rules.SeverityError}})
	writeViolations(2, goldenT2, []rules.Violation{{Rule: "fatigue_words", Target: "dường như", Limit: 1, Actual: 2, Severity: rules.SeverityWarning, Message: "“dường như” xuất hiện 2 lần"}})
	writeViolations(2, goldenT2.Add(time.Hour), nil) // chấm lại chương 2 và sạch
	writeViolations(3, goldenT3, []rules.Violation{{Rule: "fatigue_words", Target: "khẽ", Limit: 2, Actual: 5, Severity: rules.SeverityWarning}})

	st := goldenStory(t, tg, "01-story")
	res := storyReq(t, tg.g, "GET", "/x/rules/violations?last=50", adminUser(), st, "/rules/violations", "")
	if res.Code != 200 {
		t.Fatalf("violations: %d %s", res.Code, res.Body.String())
	}
	assertMockGolden(t, "rules-violations.json", res.Body.Bytes())
}
