package web

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/library"
)

func TestAudioNamingUsesCanonicalBookTitle(t *testing.T) {
	st, output := storyFixture(t, []int{1})
	if err := library.WriteStoryFile(st.Dir, library.StoryFile{Title: "RULES KỊCH BẢN: “KHI SẾP HỌC VIBECODE”", ShortTitle: "Tên riêng"}); err != nil {
		t.Fatal(err)
	}
	if err := output.Book.Save(domain.BookMetadata{Title: "Khi sếp học Vibecode", Synopsis: "Truyện công sở."}); err != nil {
		t.Fatal(err)
	}
	title, short := storyFileNames(st)
	if title != "Khi sếp học Vibecode" || short != "Tên riêng" {
		t.Fatalf("audio names = (%q, %q), want canonical title and saved shortTitle", title, short)
	}
	name := downloadFileName(st, nil, filepath.Join(st.Dir, "xuat-ban", "tap-01", "tap-1-chuong-01.mp3"), "tap-1-chuong-01.mp3")
	if name != "Tên riêng · Tập 1 · Chương 01.mp3" {
		t.Fatalf("download name = %q", name)
	}
}

func TestExportDownloadHEADUsesUnicodeFilename(t *testing.T) {
	st, _ := storyFixture(t, []int{1})
	relative := "tap-01/Khi sếp học Vibecode · Tập 1-ch1-8.mp3"
	path := filepath.Join(st.Dir, "xuat-ban", filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("audio bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"file", "download"} {
		t.Run(endpoint, func(t *testing.T) {
			query := "path"
			if endpoint == "download" {
				query = "file"
			}
			rec := storyReq(t, &Gateway{}, http.MethodHead, "/x/exports/"+endpoint+"?"+query+"="+url.QueryEscape(relative), adminUser(), st, "/exports/"+endpoint, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("HEAD = %d: %s", rec.Code, rec.Body.String())
			}
			header := rec.Header().Get("Content-Disposition")
			if !strings.Contains(header, "filename*=UTF-8''"+url.PathEscape(filepath.Base(path))) {
				t.Fatalf("Content-Disposition = %q", header)
			}
			if rec.Body.Len() != 0 {
				t.Fatalf("HEAD wrote %d body bytes", rec.Body.Len())
			}
		})
	}
}
