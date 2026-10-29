package web

import (
	"encoding/json"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/platform/identity"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// keepFixture: truyện oldbook có một bài thẩm duyệt chương 1 gồm một Cảnh báo
// (giữ được) và một mục Bắt sửa (không giữ được). Trả về cookie của người viết,
// người chỉ đọc và id hai mục.
func keepFixture(t *testing.T, tg *testGateway) (writer, reader, warningID, errorID string) {
	t.Helper()
	st, writerCookie := gateStory(t, tg)
	review := domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "polish", Summary: "Cần gọt giũa",
		Dimensions: []domain.DimensionScore{{Dimension: "hook", Score: 70, Comment: "Đã đọc"}},
		Issues: []domain.ConsistencyIssue{
			{
				Type: "style", Severity: "warning", Description: "Nhịp chậm ở đoạn giữa",
				Evidence: "đoạn 2", Quote: "đoạn 2", Chapters: []int{1},
			},
			{
				Type: "hook", Severity: "error", Description: "Kết chương an toàn",
				Evidence: "Tôi đi ngủ.", Quote: "Tôi đi ngủ.", RuleRef: "Master §7",
				Suggestion: "Mở bằng câu thoại", Chapters: []int{1}, RequiresChange: true,
			},
		},
	}
	if err := st.World.SaveReview(review); err != nil {
		t.Fatalf("review: %v", err)
	}
	rec := tg.do("GET", "/api/projects/_/stories/oldbook/reviews/1", writerCookie, "")
	if rec.Code != 200 {
		t.Fatalf("reviews/1 = %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Chapter *reviewEntryAPI `json:"chapter"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Chapter == nil || len(body.Chapter.Items) != 2 {
		t.Fatalf("items = %+v", body.Chapter)
	}
	for _, item := range body.Chapter.Items {
		switch item.Level {
		case "warning":
			warningID = item.ID
		case "error":
			errorID = item.ID
		}
	}
	if warningID == "" || errorID == "" {
		t.Fatalf("thiếu id: %+v", body.Chapter.Items)
	}
	return writerCookie, tg.login("reader", "reader-pass"), warningID, errorID
}

func keepPath(id string) string {
	return "/api/projects/_/stories/oldbook/review/items/" + id + "/keep"
}

// TestReviewItemKeepAndUnkeep pin GAP-47 / câu 14: giữ một mục Cảnh báo kèm lý do
// bắt buộc, DTO trả decision, lưu ở reviews/decisions.json, bỏ giữ xoá quyết định.
func TestReviewItemKeepAndUnkeep(t *testing.T) {
	tg := newTestGateway(t)
	defer tg.g.Close()
	writer, _, warningID, _ := keepFixture(t, tg)

	// Lý do rỗng: 400, không ghi gì.
	if w := tg.do("POST", keepPath(warningID), writer, `{"reason":"  "}`); w.Code != 400 {
		t.Fatalf("lý do rỗng = %d: %s", w.Code, w.Body)
	}
	w := tg.do("POST", keepPath(warningID), writer, `{"reason":"Cố ý: nhịp chậm để lấy không khí trước biến cố."}`)
	if w.Code != 200 {
		t.Fatalf("giữ nguyên = %d: %s", w.Code, w.Body)
	}
	body := decodeJSONBody(t, w)
	if body["state"] != "kept" || body["reason"] != "Cố ý: nhịp chậm để lấy không khí trước biến cố." {
		t.Fatalf("quyết định = %v", body)
	}
	if body["by"] == "" || body["at"] == "" {
		t.Fatalf("thiếu by/at: %v", body)
	}
	if _, err := time.Parse(time.RFC3339, body["at"].(string)); err != nil {
		t.Fatalf("at không phải RFC3339: %v", body["at"])
	}

	// DTO trả decision trên đúng mục; mục khác không có.
	rec := tg.do("GET", "/api/projects/_/stories/oldbook/reviews/1", writer, "")
	var payload struct {
		Chapter *reviewEntryAPI `json:"chapter"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	var kept, other *reviewItemAPI
	for i := range payload.Chapter.Items {
		item := &payload.Chapter.Items[i]
		if item.ID == warningID {
			kept = item
		} else {
			other = item
		}
	}
	if kept == nil || kept.Decision == nil {
		t.Fatalf("mục đã giữ thiếu decision: %+v", payload.Chapter.Items)
	}
	if kept.Decision.State != "kept" || kept.Decision.Reason == "" || kept.Decision.By == "" || kept.Decision.At == "" {
		t.Fatalf("decision = %+v", kept.Decision)
	}
	if other == nil || other.Decision != nil {
		t.Fatalf("mục khác không được có decision: %+v", other)
	}

	// Lưu ở reviews/decisions.json.
	if _, err := os.Stat(filepath.Join(tg.root, "oldbook", "output", "novel", "reviews", "decisions.json")); err != nil {
		t.Fatalf("thiếu reviews/decisions.json: %v", err)
	}
	// decisions.json không được coi là bài thẩm duyệt.
	latest := tg.do("GET", "/api/projects/_/stories/oldbook/reviews/latest", writer, "")
	var latestEntry reviewEntryAPI
	if err := json.Unmarshal(latest.Body.Bytes(), &latestEntry); err != nil {
		t.Fatalf("latest %s: %v", latest.Body, err)
	}
	if latest.Code != 200 || latestEntry.Chapter != 1 {
		t.Fatalf("reviews/latest = %d %+v", latest.Code, latestEntry)
	}

	// Bỏ giữ: 204, quyết định biến mất.
	if w := tg.do("DELETE", keepPath(warningID), writer, ""); w.Code != 204 {
		t.Fatalf("bỏ giữ = %d: %s", w.Code, w.Body)
	}
	rec = tg.do("GET", "/api/projects/_/stories/oldbook/reviews/1", writer, "")
	var after struct {
		Chapter *reviewEntryAPI `json:"chapter"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.Chapter == nil || len(after.Chapter.Items) == 0 {
		t.Fatalf("thiếu bài thẩm duyệt sau bỏ giữ: %s", rec.Body)
	}
	for _, item := range after.Chapter.Items {
		if item.ID == warningID && item.Decision != nil {
			t.Fatalf("bỏ giữ rồi mà vẫn còn decision: %+v", item.Decision)
		}
	}
	// Bỏ giữ lần hai vẫn 204 (idempotent).
	if w := tg.do("DELETE", keepPath(warningID), writer, ""); w.Code != 204 {
		t.Fatalf("bỏ giữ lần hai = %d: %s", w.Code, w.Body)
	}
}

// TestReviewItemKeepRejects pin GAP-47: chỉ Cảnh báo mới giữ được (Bắt sửa 409),
// id lạ 404, người chỉ đọc 404 (ẩn, không lộ quyền).
func TestReviewItemKeepRejects(t *testing.T) {
	tg := newTestGateway(t)
	defer tg.g.Close()
	writer, reader, _, errorID := keepFixture(t, tg)

	if w := tg.do("POST", keepPath(errorID), writer, `{"reason":"bỏ qua"}`); w.Code != 409 {
		t.Fatalf("giữ mục Bắt sửa = %d: %s", w.Code, w.Body)
	}
	if w := tg.do("POST", keepPath("ch1-deadbeef"), writer, `{"reason":"x"}`); w.Code != 404 {
		t.Fatalf("id lạ = %d: %s", w.Code, w.Body)
	}
	if w := tg.do("POST", keepPath("khong-phai-id"), writer, `{"reason":"x"}`); w.Code != 404 {
		t.Fatalf("id sai dạng = %d: %s", w.Code, w.Body)
	}
	if w := tg.do("POST", keepPath(errorID), reader, `{"reason":"x"}`); w.Code != 403 {
		t.Fatalf("người chỉ đọc = %d: %s", w.Code, w.Body)
	}
	if w := tg.do("POST", "/api/projects/_/stories/oldbook/review/items/"+errorID+"/keep", "", `{"reason":"x"}`); w.Code != 401 {
		t.Fatalf("chưa đăng nhập = %d: %s", w.Code, w.Body)
	}
	if !strings.HasPrefix(errorID, "ch1-") {
		t.Fatalf("itemId phải mang số chương: %q", errorID)
	}
}

// TestReviewItemUnkeepByCollab pin quyết định ma trận 2026-10-28 (gói 2):
// DELETE …/keep đối xứng với POST — collab được bỏ giữ mục Cảnh báo; các
// DELETE khác (story, comment…) vẫn owner-only.
func TestReviewItemUnkeepByCollab(t *testing.T) {
	tg := newTestGateway(t)
	defer tg.g.Close()
	writer, _, warningID, _ := keepFixture(t, tg)

	// Dựng collab riêng: reader chỉ có view → nâng lên collab trực tiếp qua store.
	if _, err := tg.g.accounts.UpdateUser("reader", identity.UserPatch{Projects: &map[string]identity.ProjectRole{"_": identity.ProjectCollab}}); err != nil {
		t.Fatal(err)
	}
	collab := tg.login("reader", "reader-pass")

	if w := tg.do("POST", keepPath(warningID), writer, `{"reason":"giữ để collab bỏ"}`); w.Code != 200 {
		t.Fatalf("owner giữ = %d: %s", w.Code, w.Body)
	}
	// Collab bỏ giữ → 204 (trước thay đổi: 403).
	if w := tg.do("DELETE", keepPath(warningID), collab, ""); w.Code != 204 {
		t.Fatalf("collab bỏ giữ = %d: %s", w.Code, w.Body)
	}
	// Người chỉ đọc vẫn bị chặn.
	if _, err := tg.g.accounts.UpdateUser("reader", identity.UserPatch{Projects: &map[string]identity.ProjectRole{"_": identity.ProjectView}}); err != nil {
		t.Fatal(err)
	}
	if w := tg.do("POST", keepPath(warningID), writer, `{"reason":"giữ lại"}`); w.Code != 200 {
		t.Fatalf("giữ lại = %d: %s", w.Code, w.Body)
	}
	if w := tg.do("DELETE", keepPath(warningID), collab, ""); w.Code != 403 {
		t.Fatalf("viewer bỏ giữ = %d: %s", w.Code, w.Body)
	}
	// Các DELETE khác vẫn owner-only: collab xoá story/comment → 403.
	if _, err := tg.g.accounts.UpdateUser("reader", identity.UserPatch{Projects: &map[string]identity.ProjectRole{"_": identity.ProjectCollab}}); err != nil {
		t.Fatal(err)
	}
	if w := tg.do("DELETE", "/api/projects/_/stories/oldbook", collab, ""); w.Code != 403 {
		t.Fatalf("collab xoá truyện = %d: %s", w.Code, w.Body)
	}
}
