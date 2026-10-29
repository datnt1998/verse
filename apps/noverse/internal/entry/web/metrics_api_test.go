package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/datnt1998/verse/platform/identity"
)

// metricsResponse là phần thân của GET /metrics dùng trong test.
type metricsResponse struct {
	Chapters []chapterMetrics `json:"chapters"`
}

func decodeMetrics(t *testing.T, rec *httptest.ResponseRecorder) metricsResponse {
	t.Helper()
	var out metricsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode metrics: %v\n%s", err, rec.Body.String())
	}
	return out
}

// rowsByKey đổi danh sách dòng số đo thành map để so từng dòng.
func rowsByKey(rows []metricRow) map[string]metricRow {
	out := make(map[string]metricRow, len(rows))
	for _, row := range rows {
		out[row.Key] = row
	}
	return out
}

// num là con trỏ tới một giá trị số đo (value nullable của metricRow).
func num(v float64) *float64 { return &v }

const metricsTextSimple = "“Cô đã nghe hết đoạn ghi âm đó chưa?” “Tôi nghe rồi.”"

// TestMetricsNineRowsAndValues: chín dòng số đo của gói theo đúng thứ tự, tên,
// đơn vị, chuẩn và giá trị đo được của một chương (bảng D6).
func TestMetricsNineRowsAndValues(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1})
	storyWithPack(t, st, "nu-tan-bao-thu")
	if err := s.Drafts.SaveFinalChapter(1, metricsTextSimple); err != nil {
		t.Fatal(err)
	}

	rec := storyReq(t, g, http.MethodGet, "/x/metrics?chapters=all", adminUser(), st, "/metrics", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics: %d %s", rec.Code, rec.Body)
	}
	out := decodeMetrics(t, rec)
	if len(out.Chapters) != 1 || out.Chapters[0].N != 1 {
		t.Fatalf("chapters = %+v", out.Chapters)
	}
	rows := out.Chapters[0].Metrics
	want := []string{"thoai", "cau_dai_nhat", "doan_dai_nhat", "quang_khong_thoai", "tu_thu_tuc", "tu_tieng_anh", "ngay_thu", "cum_lap", "trung_mo_dau"}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d, muốn %d: %+v", len(rows), len(want), rows)
	}
	for i, key := range want {
		if rows[i].Key != key {
			t.Fatalf("row %d = %q, muốn %q", i, rows[i].Key, key)
		}
	}
	byKey := rowsByKey(rows)
	for key, expected := range map[string]metricRow{
		"thoai":             {Name: "Tỷ lệ thoại", Value: num(100), Unit: "%", Limit: "40–55%", OK: false},
		"cau_dai_nhat":      {Name: "Câu quá 40 tiếng", Value: num(0), Unit: "câu", Limit: "0", OK: true},
		"doan_dai_nhat":     {Name: "Đoạn quá 90 tiếng", Value: num(0), Unit: "đoạn", Limit: "0", OK: true},
		"quang_khong_thoai": {Name: "Quãng không thoại quá 250 tiếng", Value: num(0), Unit: "quãng", Limit: "0", OK: true},
		"tu_thu_tuc":        {Name: "Từ thủ tục", Value: num(0), Unit: "từ", Limit: "≤ 10", OK: true},
		"tu_tieng_anh":      {Name: "Từ tiếng Anh dễ đọc sai", Value: num(0), Unit: "từ", Limit: "0", OK: true},
		"ngay_thu":          {Name: "“ngày thứ” + số", Value: num(0), Unit: "dòng", Limit: "0", OK: true},
		"cum_lap":           {Name: "Cụm 5 tiếng lặp lại", Value: num(0), Unit: "cụm", Limit: "0", OK: true},
		"trung_mo_dau":      {Name: "Mở đầu trùng truyện cũ", Value: num(0), Unit: "%", Limit: "≤ 20% · chỉ đo chương 1", OK: true},
	} {
		got := byKey[key]
		if got.Name != expected.Name || got.Unit != expected.Unit || got.Limit != expected.Limit || got.OK != expected.OK {
			t.Errorf("%s = %+v, muốn %+v", key, got, expected)
		}
		if expected.Value == nil || got.Value == nil || *got.Value != *expected.Value {
			t.Errorf("%s value = %v, muốn %v", key, got.Value, expected.Value)
		}
	}
	// Dòng "từ mệt nhiều nhất" tách khỏi chín dòng: từ có ngưỡng chặt nhất khi
	// chương không dùng từ nào trong bảng (số 0, đạt).
	tired := out.Chapters[0].TiredWord
	if tired == nil || tired.Word != "bỗng nhiên" || tired.Per1000 != 0 || !tired.OK {
		t.Fatalf("tiredWord = %+v", tired)
	}
}

// TestMetricsViolations: các phép đo vượt ngưỡng đếm đúng số câu/đoạn/quãng,
// kèm "dài nhất N tiếng"; từ mệt chọn từ có mật độ cao nhất.
func TestMetricsViolations(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1})
	storyWithPack(t, st, "nu-tan-bao-thu")
	longSentence := strings.TrimSpace(strings.Repeat("một ", 45)) + "."
	longParagraph := strings.Join([]string{
		strings.TrimSpace(strings.Repeat("một ", 32)) + ".",
		strings.TrimSpace(strings.Repeat("một ", 32)) + ".",
		strings.TrimSpace(strings.Repeat("một ", 32)) + ".",
	}, " ")
	text := strings.Join([]string{
		longSentence,
		longParagraph,
		"dường như mưa. dường như cô quên. dường như anh tới muộn.",
		"Cô ấy nói live, rồi live lần nữa.",
		"trà xanh cướp kịch bản của tôi. trà xanh cướp kịch bản của cô.",
	}, "\n\n")
	if err := s.Drafts.SaveFinalChapter(1, text); err != nil {
		t.Fatal(err)
	}

	rec := storyReq(t, g, http.MethodGet, "/x/metrics", adminUser(), st, "/metrics", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics: %d %s", rec.Code, rec.Body)
	}
	out := decodeMetrics(t, rec)
	if len(out.Chapters) != 1 {
		t.Fatalf("chapters = %+v", out.Chapters)
	}
	byKey := rowsByKey(out.Chapters[0].Metrics)
	for key, expected := range map[string]metricRow{
		"thoai":             {Value: num(0), Limit: "40–55%", OK: false},
		"cau_dai_nhat":      {Value: num(1), Limit: "0", OK: false, Detail: "dài nhất 45 tiếng"},
		"doan_dai_nhat":     {Value: num(1), Limit: "0", OK: false, Detail: "dài nhất 96 tiếng"},
		"quang_khong_thoai": {Value: num(0), Limit: "0", OK: true},
		"tu_tieng_anh":      {Value: num(2), Limit: "0", OK: false},
		"cum_lap":           {Value: num(1), Limit: "0", OK: false},
	} {
		got := byKey[key]
		if expected.Value == nil || got.Value == nil || *got.Value != *expected.Value || got.OK != expected.OK {
			t.Errorf("%s = %+v, muốn value=%v ok=%v", key, got, expected.Value, expected.OK)
		}
		if got.Limit != expected.Limit {
			t.Errorf("%s limit = %q, muốn %q", key, got.Limit, expected.Limit)
		}
		if got.Detail != expected.Detail {
			t.Errorf("%s detail = %q, muốn %q", key, got.Detail, expected.Detail)
		}
	}
	tired := out.Chapters[0].TiredWord
	if tired == nil || tired.Word != "dường như" || tired.Per1000 != 17.14 || tired.Limit != 1.33 || tired.OK {
		t.Fatalf("tiredWord = %+v", tired)
	}
}

// TestMetricsSeverityAndRef: mỗi dòng lấy mức nặng từ đúng dòng luật của gói
// (Câu 16) — quãng không thoại là [LỖI] ở Master §5, còn lại là [CẢNH BÁO].
func TestMetricsSeverityAndRef(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1})
	storyWithPack(t, st, "nu-tan-bao-thu")
	if err := s.Drafts.SaveFinalChapter(1, metricsTextSimple); err != nil {
		t.Fatal(err)
	}
	rows := rowsByKey(decodeMetrics(t, storyReq(t, g, http.MethodGet, "/x/metrics", adminUser(), st, "/metrics", "")).Chapters[0].Metrics)
	for key, want := range map[string]metricRef{
		"thoai":             {Severity: "warning", Ref: "Master §13"},
		"cau_dai_nhat":      {Severity: "warning", Ref: "Master §15.1"},
		"doan_dai_nhat":     {Severity: "warning", Ref: "Master §15.1"},
		"quang_khong_thoai": {Severity: "error", Ref: "Master §5"},
		"tu_thu_tuc":        {Severity: "warning", Ref: "Master §14"},
		"tu_tieng_anh":      {Severity: "warning", Ref: "Master §15.3"},
		"ngay_thu":          {Severity: "warning", Ref: "Master §15.3"},
		"cum_lap":           {Severity: "error", Ref: "Master §13"},
		"trung_mo_dau":      {Severity: "warning", Ref: "Master §17"},
	} {
		got := metricRef{Severity: rows[key].Severity, Ref: rows[key].Ref}
		if got != want {
			t.Errorf("%s severity/ref = %+v, muốn %+v", key, got, want)
		}
	}

	// Bộ C ghi đè §14: từ thủ tục là [LỖI] của riêng thể loại đó.
	storyWithPack(t, st, "nien-dai-van")
	rows = rowsByKey(decodeMetrics(t, storyReq(t, g, http.MethodGet, "/x/metrics", adminUser(), st, "/metrics", "")).Chapters[0].Metrics)
	got := metricRef{Severity: rows["tu_thu_tuc"].Severity, Ref: rows["tu_thu_tuc"].Ref}
	if want := (metricRef{Severity: "error", Ref: "Bộ C §14"}); got != want {
		t.Fatalf("bộ C từ thủ tục = %+v, muốn %+v", got, want)
	}
}

// TestMetricsOpeningOnlyChapterOne: mở đầu trùng chỉ đo ở chương 1; chương khác
// không có số (null) và giữ nguyên chuẩn để giao diện hiện "—".
func TestMetricsOpeningOnlyChapterOne(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1, 2})
	storyWithPack(t, st, "nu-tan-bao-thu")
	for _, n := range []int{1, 2} {
		if err := s.Drafts.SaveFinalChapter(n, metricsTextSimple); err != nil {
			t.Fatal(err)
		}
	}
	out := decodeMetrics(t, storyReq(t, g, http.MethodGet, "/x/metrics?chapters=all", adminUser(), st, "/metrics", ""))
	if len(out.Chapters) != 2 {
		t.Fatalf("chapters = %+v", out.Chapters)
	}
	first := rowsByKey(out.Chapters[0].Metrics)["trung_mo_dau"]
	if first.Value == nil {
		t.Fatalf("chương 1 phải có số mở đầu: %+v", first)
	}
	second := rowsByKey(out.Chapters[1].Metrics)["trung_mo_dau"]
	if second.Value != nil {
		t.Fatalf("chương 2 không được có số mở đầu: %+v", second)
	}
	if !second.OK || second.Limit != "≤ 20% · chỉ đo chương 1" {
		t.Fatalf("chương 2 mở đầu = %+v", second)
	}
}

// TestMetricsScope: mặc định chỉ ba chương cuối; "all" trả hết; tham số lạ 400.
func TestMetricsScope(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1, 2, 3, 4, 5})
	storyWithPack(t, st, "nu-tan-bao-thu")
	for n := 1; n <= 5; n++ {
		if err := s.Drafts.SaveFinalChapter(n, metricsTextSimple); err != nil {
			t.Fatal(err)
		}
	}
	last3 := decodeMetrics(t, storyReq(t, g, http.MethodGet, "/x/metrics", adminUser(), st, "/metrics", ""))
	if len(last3.Chapters) != 3 || last3.Chapters[0].N != 3 || last3.Chapters[2].N != 5 {
		t.Fatalf("last3 = %+v", last3.Chapters)
	}
	all := decodeMetrics(t, storyReq(t, g, http.MethodGet, "/x/metrics?chapters=all", adminUser(), st, "/metrics", ""))
	if len(all.Chapters) != 5 {
		t.Fatalf("all = %d chương", len(all.Chapters))
	}
	if rec := storyReq(t, g, http.MethodGet, "/x/metrics?chapters=bogus", adminUser(), st, "/metrics", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("chapters=bogus → %d, muốn 400", rec.Code)
	}
}

// TestMetricsReaderAllowedAndNoPack: người chỉ đọc xem được; truyện không dùng gói
// thì không có phép đo nào (danh sách rỗng, không bịa số).
func TestMetricsReaderAllowedAndNoPack(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, []int{1})
	if err := s.Drafts.SaveFinalChapter(1, metricsTextSimple); err != nil {
		t.Fatal(err)
	}
	reader := identity.User{Username: "reader", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{st.Project: identity.ProjectView}}
	out := decodeMetrics(t, storyReq(t, g, http.MethodGet, "/x/metrics", reader, st, "/metrics", ""))
	if len(out.Chapters) != 0 {
		t.Fatalf("truyện không gói: chapters = %+v", out.Chapters)
	}

	storyWithPack(t, st, "nu-tan-bao-thu")
	out = decodeMetrics(t, storyReq(t, g, http.MethodGet, "/x/metrics", reader, st, "/metrics", ""))
	if len(out.Chapters) != 1 {
		t.Fatalf("người chỉ đọc: chapters = %+v", out.Chapters)
	}
}
