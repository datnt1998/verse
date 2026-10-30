package retention

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/fsio"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// Nhãn nhóm trên bảng xem trước.
const (
	labelTemp      = "Tệp tạm khi dựng và tải video"
	labelExportReq = "Bản ghi xuất đã xong"
	labelStaged    = "Bản xuất chưa chốt"
	labelMedia     = "Media không còn video hay template nào dùng"
)

// exportRequestRetention: bản ghi yêu cầu xuất đã xong quá 3 ngày chỉ còn là
// lịch sử — xoá cùng profile Chrome của nó.
const exportRequestRetention = 3 * 24 * time.Hour

// stagedExportMaxAge: bản xuất "staged" quá 24 giờ không thể còn thuộc một
// lần tải lên đang chạy.
const stagedExportMaxAge = 24 * time.Hour

// requestSchema là subset của một bản ghi yêu cầu xuất.
type exportRequest struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
	FinishedAt string `json:"finishedAt"`
}

// terminal báo trạng thái yêu cầu xuất đã dừng hẳn.
func (r exportRequest) terminal() bool {
	switch r.Status {
	case "done", "failed", "cancelled", "stale":
		return true
	}
	return false
}

// finishedAt đọc mốc kết thúc (không có thì mốc tạo) — mốc để tính hạn.
func (r exportRequest) finishedAt() (time.Time, bool) {
	text := r.FinishedAt
	if text == "" {
		text = r.CreatedAt
	}
	t, err := time.Parse(time.RFC3339, text)
	return t, err == nil
}

// requestFiles liệt kê mọi bản ghi yêu cầu xuất trong exports/requests.
func (s *Service) requestFiles() ([]string, map[string]exportRequest) {
	dir := filepath.Join(s.deps.Roots.Root(), "exports", "requests")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	byID := map[string]exportRequest{}
	var paths []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		var req exportRequest
		if !readJSON(path, &req) {
			continue // bản ghi hỏng: không đoán, để người dùng tự xoá
		}
		if req.ID == "" {
			req.ID = strings.TrimSuffix(e.Name(), ".json")
		}
		paths = append(paths, path)
		byID[req.ID] = req
	}
	return paths, byID
}

// tempCandidates: thư mục tạm app tạo dưới TmpDir mà owner đã chết (quá 6
// giờ). Thư mục của một lần vẽ ảnh chưa kết thúc là phiên phục hồi — bỏ qua.
func (s *Service) tempCandidates(ctx context.Context) ([]Candidate, error) {
	now := s.now()
	stale := tools.ListStaleTempDirs(s.deps.TmpDir, tools.AppTempPrefixes(), tools.StaleTempDirAge, now)
	if len(stale) == 0 {
		return nil, nil
	}
	// Receipt lỗi đọc được nhưng scratch dirs không biết được → fail
	// closed cho mọi thư mục call-* (có thể là phiên ảnh đã trả tiền).
	protected := map[string]bool{}
	receiptsFailed := false
	if s.deps.Receipts != nil {
		dirs, err := s.deps.Receipts.ScratchDirs(ctx)
		if err != nil {
			receiptsFailed = true
		} else {
			for _, dir := range dirs {
				protected[resolvePath(dir)] = true
			}
		}
	}
	var out []Candidate
	var unsafe bool
	for _, dir := range stale {
		if protected[resolvePath(dir)] {
			continue
		}
		// call-* là phiên gọi (ảnh/story/metadata): không chứng minh được
		// "không phải phiên phục hồi" khi receipt lỗi thì giữ lại.
		callScratch := receiptsFailed && strings.Contains(filepath.Base(dir), "call-")
		if callScratch {
			unsafe = true
			continue
		}
		path := dir
		out = append(out, Candidate{
			Item:   Item{Kind: "temp-dir", Label: labelTemp, Path: path, Bytes: fsio.Bytes(path)},
			Remove: func(context.Context) error { return os.RemoveAll(path) },
		})
	}
	if unsafe {
		return out, fmt.Errorf("không đọc được phiên vẽ ảnh đang dở — giữ mọi thư mục call-* lại")
	}
	return out, nil
}

// codexHomeCandidates: home Codex dùng một lần (`cache/codex-home/call-*`).
// Mỗi cái giữ một bản đăng nhập; không cái nào sống quá lần gọi của nó.
func (s *Service) codexHomeCandidates(context.Context) ([]Candidate, error) {
	dir := filepath.Join(s.deps.ConfigDir, "cache", "codex-home")
	stale := tools.ListStaleTempDirs(dir, []string{"call-"}, tools.StaleTempDirAge, s.now())
	var out []Candidate
	for _, path := range stale {
		path := path
		out = append(out, Candidate{
			Item:   Item{Kind: "codex-home", Label: labelTemp, Path: path, Bytes: fsio.Bytes(path)},
			Remove: func(context.Context) error { return os.RemoveAll(path) },
		})
	}
	return out, nil
}

// exportRequestCandidates: bản ghi yêu cầu xuất đã xong quá hạn + profile
// Chrome của mọi yêu cầu đã dừng (hoặc không còn bản ghi).
func (s *Service) exportRequestCandidates(context.Context) ([]Candidate, error) {
	now := s.now()
	paths, _ := s.requestFiles()
	live := map[string]bool{}
	var out []Candidate
	for _, requestPath := range paths {
		path := requestPath
		var req exportRequest
		if !readJSON(path, &req) {
			continue
		}
		if req.ID == "" {
			req.ID = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		if !req.terminal() {
			live[req.ID] = true
			continue
		}
		finished, ok := req.finishedAt()
		if !ok || now.Sub(finished) <= exportRequestRetention {
			live[req.ID] = true // chưa quá hạn: giữ nguyên bằng chứng
			continue
		}
		out = append(out, Candidate{
			Item:   Item{Kind: "export-request", Label: labelExportReq, Path: path, Bytes: fsio.Bytes(path)},
			Remove: func(context.Context) error { return os.Remove(path) },
			Validator: func(context.Context) bool {
				var re exportRequest
				if !readJSON(path, &re) {
					return false // đã đổi/hỏng: giữ lại, lần preview sau dò tiếp
				}
				if !re.terminal() {
					return false
				}
				fin, ok := re.finishedAt()
				return ok && s.now().Sub(fin) > exportRequestRetention
			},
		})
	}
	profiles := filepath.Join(s.deps.Roots.Root(), "exports", "profiles")
	entries, err := os.ReadDir(profiles)
	if err != nil {
		return out, nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		if live[id] {
			continue // yêu cầu còn chạy (hoặc chưa quá hạn): profile đang được dùng
		}
		path := filepath.Join(profiles, id)
		out = append(out, Candidate{
			Item:   Item{Kind: "export-request", Label: labelExportReq, Path: path, Bytes: fsio.Bytes(path)},
			Remove: func(context.Context) error { return os.RemoveAll(path) },
		})
	}
	return out, nil
}

// stagedExport là subset của bản ghi một lần xuất trong project.
type stagedExport struct {
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// stagedExportCandidates: bản xuất chưa chốt (`status=staged`) quá 24 giờ.
func (s *Service) stagedExportCandidates(context.Context) ([]Candidate, error) {
	now := s.now()
	exports := filepath.Join(s.deps.Roots.Root(), "exports")
	videos, err := os.ReadDir(exports)
	if err != nil {
		return nil, nil
	}
	var out []Candidate
	for _, v := range videos {
		if !v.IsDir() || v.Name() == "requests" || v.Name() == "profiles" {
			continue
		}
		dir := filepath.Join(exports, v.Name())
		records, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, rec := range records {
			if !rec.IsDir() {
				continue
			}
			recordDir := filepath.Join(dir, rec.Name())
			meta := filepath.Join(recordDir, "export.json")
			var ex stagedExport
			if !readJSON(meta, &ex) || ex.Status != "staged" {
				continue
			}
			created, err := time.Parse(time.RFC3339, ex.CreatedAt)
			if err != nil || now.Sub(created) <= stagedExportMaxAge {
				continue
			}
			out = append(out, Candidate{
				Item:   Item{Kind: "staged-export", Label: labelStaged, Path: recordDir, Bytes: fsio.Bytes(recordDir)},
				Remove: func(context.Context) error { return os.RemoveAll(recordDir) },
				Validator: func(context.Context) bool {
					var re stagedExport
					if !readJSON(meta, &re) || re.Status != "staged" {
						return false // đã chốt/đổi: không được xoá
					}
					created, err := time.Parse(time.RFC3339, re.CreatedAt)
					return err == nil && s.now().Sub(created) > stagedExportMaxAge
				},
			})
		}
	}
	return out, nil
}

// resolvePath chuẩn hoá đường dẫn để so (symlink của TmpDir trên macOS).
func resolvePath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}
