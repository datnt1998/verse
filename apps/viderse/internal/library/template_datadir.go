package library

// Nhập template thẳng từ thư mục dữ liệu của bản cũ — không cần chạy công cụ
// trước đây, không cần tệp xuất. Thư mục đó gồm:
//
//	<data>/app.db                        registry template (SQLite)
//	<data>/editor-workspace/projects/<id>/project.json
//	<data>/editor-workspace/media/<id>/  metadata.json + tệp nguồn
//
// Registry là nguồn duy nhất cho biết project nào là template, nên phải đọc
// `app.db`. Đọc trên BẢN SAO: app.db + -wal + -shm được chép sang thư mục tạm
// rồi mới mở, thư mục dữ liệu của người dùng không bị mở ghi, không bị khoá.
//
// Mỗi hàng registry thành một TemplateBundle y như bundle đọc từ tệp
// `.ytf-template.zip`, nên phần quy đổi và ghi chỉ có một đường.

import (
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/ncruces/go-sqlite3/driver" // driver "sqlite3" cho database/sql
	_ "github.com/ncruces/go-sqlite3/embed"  // bộ máy SQLite WASM, không cần cgo
)

// maxDataDirDBBytes là trần app.db chép sang bản tạm (thư viện vài nghìn
// template vẫn dưới mức này).
const maxDataDirDBBytes = 1 << 30

// OpenTemplateDataDir đọc registry template của thư mục dữ liệu cũ và trả một
// bundle cho mỗi template, theo thứ tự tạo. Lỗi của một template không chặn
// các template khác: hàng hỏng thành bundle nil kèm lỗi ở TemplateDataDirItem.
func OpenTemplateDataDir(dir string) ([]TemplateDataDirItem, error) {
	workspace := filepath.Join(dir, "editor-workspace")
	if !isDirectory(workspace) {
		return nil, invalidInput("Thư mục không có editor-workspace",
			"Chỉ đường dẫn tới thư mục dữ liệu của bản cũ (có app.db và editor-workspace)")
	}
	ids, err := readTemplateRegistry(filepath.Join(dir, "app.db"))
	if err != nil {
		return nil, err
	}
	items := make([]TemplateDataDirItem, 0, len(ids))
	for _, id := range ids {
		projectDir := filepath.Join(workspace, "projects", id)
		bundle, err := openTemplateDir(projectDir)
		if err != nil {
			items = append(items, TemplateDataDirItem{ProjectID: id, Name: id, Err: err})
			continue
		}
		items = append(items, TemplateDataDirItem{ProjectID: id, Name: bundle.Name, Bundle: bundle})
	}
	if len(items) == 0 {
		return nil, invalidInput("Thư mục dữ liệu cũ không có mẫu video nào",
			"Kiểm tra app.db còn hàng trong bảng templates")
	}
	return items, nil
}

// TemplateDataDirItem là một template đọc từ thư mục dữ liệu cũ. Bundle nil
// nghĩa là template đó hỏng (Err nói vì sao) — các template khác vẫn nhập được.
type TemplateDataDirItem struct {
	ProjectID string
	Name      string
	Bundle    *TemplateBundle
	Err       error
}

// readTemplateRegistry trả project_id của mọi template trong `app.db`, đọc
// trên bản sao trong thư mục tạm.
func readTemplateRegistry(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, invalidInput("Thư mục không có app.db",
			"Chỉ đường dẫn tới thư mục dữ liệu của bản cũ (có app.db và editor-workspace)")
	}
	if info.Size() > maxDataDirDBBytes {
		return nil, invalidInput("app.db quá lớn để đọc", "Chỉ đường dẫn tới thư mục dữ liệu của bản cũ")
	}
	dir, err := os.MkdirTemp("", "viderse-template-registry-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := copyFileIfExists(path+suffix, filepath.Join(dir, filepath.Base(path)+suffix)); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite3", filepath.Join(dir, filepath.Base(path)))
	if err != nil {
		return nil, invalidInput("Không mở được app.db của thư mục dữ liệu cũ", "Chép lại thư mục dữ liệu rồi nhập lại")
	}
	defer db.Close()
	rows, err := db.Query(`select project_id from templates order by created_at`)
	if err != nil {
		return nil, invalidInput("app.db không có bảng templates",
			"Chỉ đường dẫn tới thư mục dữ liệu của bản cũ (có app.db và editor-workspace)")
	}
	defer rows.Close()
	ids := make([]string, 0, 16)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		id = strings.TrimSpace(id)
		if id == "" || strings.ContainsAny(id, `/\`) || id == "." || id == ".." {
			continue
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// copyFileIfExists chép một tệp; tệp không có thì bỏ qua (app.db không có -wal
// là bình thường).
func copyFileIfExists(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, io.LimitReader(source, maxDataDirDBBytes)); err != nil {
		_ = target.Close()
		return err
	}
	return target.Close()
}

// isDirectory báo đường dẫn là thư mục.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ImportBundles nhập một lô bundle đã đọc sẵn (nguồn: thư mục dữ liệu cũ).
// Một template hỏng không dừng lô: dòng của nó có OK=false kèm lý do.
func (s *Service) ImportBundles(ctx context.Context, items []TemplateDataDirItem, opts ImportOptions) []ImportResult {
	results := make([]ImportResult, 0, len(items))
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			break
		}
		if item.Bundle == nil {
			results = append(results, ImportResult{Source: item.ProjectID, Name: item.Name, Error: errorText(item.Err)})
			continue
		}
		result, err := s.ImportTemplate(ctx, item.Bundle, opts)
		if err != nil {
			results = append(results, ImportResult{Source: item.ProjectID, Name: item.Name, Error: errorText(err)})
			continue
		}
		result.Source = item.ProjectID
		results = append(results, *result)
	}
	return results
}
