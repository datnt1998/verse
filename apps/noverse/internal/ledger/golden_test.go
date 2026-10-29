package ledger_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/ledger"
)

// Golden ledger-check*.json của web/mock: mock từng tự cài đặt lại bốn check
// của CheckValues (dup tổ hợp, khoảng cách, lạm dụng trục 3/4/7, mở đầu); giờ
// phát lại đúng Result này. `go test ./internal/ledger -update` ghi lại sau khi
// chủ đích đổi CheckValues hoặc bảng truyện; handler web so byte với
// ledger-check.json (assertMockGolden trong TestMockGoldenLedger).

var updateMockGoldens = flag.Bool("update", false, "ghi lại golden của mock dưới web/mock/golden")

func assertMockGolden(t *testing.T, name string, v any) {
	t.Helper()
	var pretty bytes.Buffer
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	path := filepath.Join("..", "..", "web", "mock", "golden", name)
	if *updateMockGoldens {
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
		t.Fatalf("read %s: %v (chạy go test -update để tạo)", path, err)
	}
	if !bytes.Equal(pretty.Bytes(), want) {
		t.Fatalf("%s lệch output CheckValues; nếu chủ đích, chạy -update\ngot:\n%s", path, pretty.String())
	}
}

// goldenLedgerPrior là tập "truyện trước" mới nhất trước — đảo tên thư mục của
// mockLedgerSeed trong mock_golden_test.go (web). Ứng viên (02-chua-co-ten) có
// sổ cái nhưng handler loại nó khỏi tập trước vì brief trùng, nên không ở đây.
func goldenLedgerPrior() []ledger.Story {
	return []ledger.Story{
		{ID: "proj/thien-kim", Title: "Thiên Kim Thật Trở Về, Cả Họ Quỳ Trước Cổng", Values: ledger.Values{"A thiên kim", "Hào môn", "Y tá", "Em nuôi", "Mẹ già", "Bản xét nghiệm", "Đám giỗ", "Mỏ hỗn sắc", "Khoảnh khắc tỉnh lại", "Đoàn tụ"}, Opening: "Khoảnh khắc tỉnh lại"},
		{ID: "proj/me-ke-tiem-may", Title: "Mẹ Kế Cướp Tiệm May, Tôi Trọng Sinh Lấy Lại Từng Đường Kim", Values: ledger.Values{"A trọng sinh", "Khu chợ", "Thợ may", "Mẹ kế", "Món đồ của mẹ", "Sổ sách viết tay", "Chợ phiên", "Điềm tĩnh lạnh", "Khoảnh khắc chết", "Rời đi"}, Opening: "Khoảnh khắc chết"},
		{ID: "proj/me-chong", Title: "Mẹ Chồng Đuổi Tôi Đi, Tôi Thành Chủ Chuỗi Tiệm Bánh", Values: ledger.Values{"B", "Nhà hàng", "Đầu bếp", "Mẹ chồng", "Con nhỏ", "Ghi âm", "Phiên toà", "Ấm, khóc được", "Một quyết định", "Sự nghiệp riêng"}, Opening: "Một quyết định"},
		{ID: "proj/hidden-story", Title: "Truyện riêng không được xem", Values: ledger.Values{"Bí mật", "Bí mật", "Bí mật", "Bí mật", "Bí mật", "Bí mật", "Bí mật", "Bí mật", "Bí mật", "Bí mật"}, Opening: "Bí mật"},
		{ID: "proj/ganh-xoi", Title: "Năm Tám Mươi, Tôi Nuôi Cả Nhà Bằng Gánh Xôi", Values: ledger.Values{"C-Nữ", "Thập niên 80", "Bán xôi", "Họ hàng", "Bà nội", "Sổ sách viết tay", "Chợ phiên", "Ấm, khóc được", "Một quyết định", "Đoàn tụ"}, Opening: "Một quyết định"},
		{ID: "proj/em-gai-nuoi", Title: "Em Gái Nuôi Mặc Váy Cưới Của Tôi Lên Livestream", Values: ledger.Values{"A trọng sinh", "Công ty", "Thợ may", "Em nuôi", "Con vật nuôi", "Bản nháp có dấu thời gian", "Livestream", "Hài", "Khoảnh khắc tỉnh lại", "Sự nghiệp riêng"}, Opening: "Khoảnh khắc tỉnh lại"},
		{ID: "proj/dong-nghiep", Title: "Đồng Nghiệp Cướp Công, Tôi Trình Bày Lại Trước Đại Hội Cổ Đông", Values: ledger.Values{"A trọng sinh", "Công ty", "Kế toán", "Đồng nghiệp", "Mẹ già", "Email gốc", "Đại hội cổ đông", "Điềm tĩnh lạnh", "Khoảnh khắc chết", "Sự nghiệp riêng"}, Opening: "Khoảnh khắc chết"},
		{ID: "proj/dam-gio", Title: "Đám Giỗ Cả Họ, Tôi Đọc Di Chúc Của Bà", Values: ledger.Values{"C-Nữ", "Thập niên 80", "Thợ mộc", "Họ hàng", "Bà nội", "Di chúc", "Đám giỗ", "Ấm, khóc được", "Câu thoại bất công", "Đoàn tụ"}, Opening: "Câu thoại bất công"},
		{ID: "proj/chong-cu-quan-pho", Title: "Chồng Cũ Hối Hận Khi Tôi Mở Quán Phở Đầu Ngõ", Values: ledger.Values{"B", "Phố cổ", "Đầu bếp", "Chồng", "Con nhỏ", "Ghi âm", "Đám giỗ", "Ấm, khóc được", "Một quyết định", "Tình cảm mới"}, Opening: "Một quyết định"},
		{ID: "proj/ban-than", Title: "Bạn Thân Giật Chồng, Tôi Lật Sổ Đỏ Giữa Lễ Đính Hôn", Values: ledger.Values{"B", "Làng chài", "Kế toán", "Bạn thân", "Con nhỏ", "Sổ đỏ", "Lễ đính hôn", "Điềm tĩnh lạnh", "Khoảnh khắc chết", "Rời đi"}, Opening: "Khoảnh khắc chết"},
		{ID: "proj/01-tra-xanh-cuop-kich-ban", Title: "Trà Xanh Cướp Kịch Bản Của Tôi, Tôi Lên Show Thực Tế Đọc Bản Gốc", Values: ledger.Values{"A showbiz", "Đoàn phim", "Biên kịch", "Bạn diễn", "Mẹ già", "Bản ghi âm gốc", "Livestream", "Mỏ hỗn sắc", "Câu thoại bất công", "Sự nghiệp riêng"}, Opening: "Câu thoại bất công"},
	}
}

func withoutStory(prior []ledger.Story, id string) []ledger.Story {
	out := []ledger.Story{}
	for _, s := range prior {
		if s.ID != "proj/"+id {
			out = append(out, s)
		}
	}
	return out
}

// Giá trị ứng viên "chua-co-ten": trùng 4 trục chính của me-ke (dup) và vấp
// overuse trục 3 ("Thợ may" đã có 2 truyện).
var goldenLedgerCandidate = ledger.Values{
	"A trọng sinh", "Khu chợ", "Thợ may", "Mẹ kế", "Mẹ già", "Sổ sách viết tay", "Lễ đính hôn", "Mỏ hỗn sắc", "Khoảnh khắc tỉnh lại", "Sự nghiệp riêng",
}

// Giá trị đã sửa: đổi trục 4 (Mẹ kế → Chủ tiệm) phá trùng tổ hợp.
var goldenLedgerCandidateFixed = ledger.Values{
	"A trọng sinh", "Khu chợ", "Thợ may", "Chủ tiệm", "Mẹ già", "Sổ sách viết tay", "Lễ đính hôn", "Mỏ hỗn sắc", "Khoảnh khắc tỉnh lại", "Sự nghiệp riêng",
}

func TestMockGoldenLedgerChecks(t *testing.T) {
	prior := goldenLedgerPrior()
	assertMockGolden(t, "ledger-check.json", ledger.CheckValues(goldenLedgerCandidate, prior))
	assertMockGolden(t, "ledger-check-fixed.json", ledger.CheckValues(goldenLedgerCandidateFixed, prior))

	// Truyện trùng tổ hợp bị ẩn khỏi tầm nhì: lạm dụng trục 3 vẫn còn
	// ("Thợ may" ở em-gai-nuoi), dup và khoảng cách đổi theo tập còn lại.
	assertMockGolden(t, "ledger-check-dup-hidden.json", ledger.CheckValues(goldenLedgerCandidate, withoutStory(prior, "me-ke-tiem-may")))
}
