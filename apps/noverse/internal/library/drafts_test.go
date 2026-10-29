package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newDraftProject 建一个带 project.json 与 kich_ban 的项目。
func newDraftProject(t *testing.T) (*Library, string) {
	t.Helper()
	lib, root := openTemp(t)
	mustWrite(t, filepath.Join(root, "du-an", "project.json"), `{"name":"Dự Án"}`)
	if err := os.MkdirAll(filepath.Join(root, "du-an", "kich_ban"), 0o755); err != nil {
		t.Fatal(err)
	}
	return lib, root
}

func TestCommitDraftMovesDirAndWritesStory(t *testing.T) {
	lib, root := newDraftProject(t)
	dir, err := lib.CreateDraft("du-an", "20260101-101010-abcdef")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "output", "novel", "chapters", "01.md"), "Chương một")
	if err := os.MkdirAll(filepath.Join(root, "du-an", "kich_ban", "01-co-san"), 0o755); err != nil {
		t.Fatal(err)
	}

	st, tempName, err := lib.CommitDraft("du-an", "20260101-101010-abcdef", "Trà Xanh Cướp Kịch Bản Của Tôi")
	if err != nil {
		t.Fatal(err)
	}
	if tempName {
		t.Fatal("tên thật không được là tempName")
	}
	if st.Story != "02-tra-xanh-cuop-kich-ban" {
		t.Fatalf("story dir: %s", st.Story)
	}
	if _, err := os.Stat(filepath.Join(st.Dir, "output", "novel", "chapters", "01.md")); err != nil {
		t.Fatal("engine output không đi theo thư mục")
	}
	if _, err := os.Stat(filepath.Join(root, "du-an", ".ainovel", "drafts", "20260101-101010-abcdef")); !os.IsNotExist(err) {
		t.Fatal("thư mục nháp phải biến mất sau commit")
	}
	f, err := LoadStoryFile(st.Dir)
	if err != nil || f == nil {
		t.Fatalf("story.json: %v %v", f, err)
	}
	if f.Title != "Trà Xanh Cướp Kịch Bản Của Tôi" || f.Status != "idea" {
		t.Fatalf("story.json: %+v", f)
	}

	// 无 tên → NN-truyen-moi + tempName.
	dir2, err := lib.CreateDraft("du-an", "20260102-101010-abcdef")
	if err != nil {
		t.Fatal(err)
	}
	_ = dir2
	st2, temp, err := lib.CommitDraft("du-an", "20260102-101010-abcdef", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if !temp || !strings.HasSuffix(st2.Story, "-truyen-moi") {
		t.Fatalf("temp story: %+v temp=%v", st2, temp)
	}
}

func TestCommitDraftRejectsBadIDsAndLegacy(t *testing.T) {
	lib, _ := newDraftProject(t)
	if _, _, err := lib.CommitDraft("du-an", "../escape", "x"); err == nil {
		t.Fatal("id có ../ phải bị từ chối")
	}
	if _, _, err := lib.CommitDraft("du-an", "khong-co", "x"); err == nil {
		t.Fatal("nháp không tồn tại phải lỗi")
	}
	if _, _, err := lib.CommitDraft(LegacyProjectID, "20260101-101010-abcdef", "x"); err == nil {
		t.Fatal("dự án ẩn danh không hỗ trợ")
	}
	// Ngay cả khi thư mục nháp tồn tại dưới gốc, dự án "_" vẫn bị từ chối.
	if err := os.MkdirAll(filepath.Join(lib.Root, ".ainovel", "drafts", "20260101-101010-abcdef"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := lib.CommitDraft(LegacyProjectID, "20260101-101010-abcdef", "x"); err == nil {
		t.Fatal("dự án ẩn danh không được commit nháp")
	}
}

func TestCleanupDraftsRemovesOnlyOld(t *testing.T) {
	lib, root := newDraftProject(t)
	oldDir, err := lib.CreateDraft("du-an", "20250101-101010-aaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	freshDir, err := lib.CreateDraft("du-an", "20260101-101010-bbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(oldDir, old, old); err != nil {
		t.Fatal(err)
	}
	// 名字不合法的目录不能被当草稿删掉。
	junk := filepath.Join(root, "du-an", ".ainovel", "drafts", "Giữ Lại")
	if err := os.MkdirAll(junk, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(junk, old, old); err != nil {
		t.Fatal(err)
	}

	n, err := lib.CleanupDrafts(7 * 24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("removed %d, muốn 1", n)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatal("nháp cũ phải bị xoá")
	}
	if _, err := os.Stat(freshDir); err != nil {
		t.Fatal("nháp mới phải còn")
	}
	if _, err := os.Stat(junk); err != nil {
		t.Fatal("thư mục không đúng chuẩn id không được xoá")
	}
}
