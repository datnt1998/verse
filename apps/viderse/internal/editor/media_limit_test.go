package editor

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/uploadlimit"
)

// TestMediaLimitHostedCapAndEnvPrecedence pins the order of precedence: an
// explicit YTF_EDITOR_MAX_* override always wins, otherwise the built-in
// ceiling is lowered to the public upload limit when PUBLIC_URL is set.
func TestMediaLimitHostedCapAndEnvPrecedence(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	t.Setenv("YTF_EDITOR_MAX_VIDEO_BYTES", "")
	if got := MediaLimit("video"); got != 20<<30 {
		t.Fatalf("video local = %d", got)
	}

	t.Setenv("PUBLIC_URL", "https://vid.example")
	if got := MediaLimit("video"); got != uploadlimit.PublicMaxBytes {
		t.Fatalf("video hosted = %d, muốn %d", got, uploadlimit.PublicMaxBytes)
	}
	if got := MediaLimit("audio"); got != uploadlimit.PublicMaxBytes {
		t.Fatalf("audio hosted = %d, muốn %d", got, uploadlimit.PublicMaxBytes)
	}
	// Ảnh vốn đã dưới trần công khai: giữ nguyên.
	if got := MediaLimit("image"); got != 50<<20 {
		t.Fatalf("image hosted = %d", got)
	}

	// Override tường minh vẫn thắng, kể cả khi lớn hơn trần công khai.
	t.Setenv("YTF_EDITOR_MAX_VIDEO_BYTES", "1048576")
	if got := MediaLimit("video"); got != 1<<20 {
		t.Fatalf("video override = %d", got)
	}
}

// TestMediaImportOverPublicLimitNamesTheLimit pins the refusal copy: over the
// internet the payload_too_large error must state the ceiling in Vietnamese
// instead of the editor's generic English sentence.
func TestMediaImportOverPublicLimitNamesTheLimit(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	t.Setenv("PUBLIC_URL", "https://vid.example")
	t.Setenv("YTF_EDITOR_MAX_IMAGE_BYTES", "1048576")
	b, e := os.ReadFile(filepath.Join("testdata", "__fixtures__", "image.png"))
	if e != nil {
		t.Fatal(e)
	}
	// 2 MiB khai báo là ảnh, trần 1 MiB: bị chặn ở bước ghi tạm.
	body := bytes.NewReader(append(b, make([]byte, 2<<20)...))
	_, e = s.ImportMedia(ctx, ImportMediaOptions{FileName: "image.png", DeclaredMIME: "image/png", Body: body})
	mediaErrorCode(t, e, "payload_too_large")
	if !strings.Contains(e.Error(), "Tải lên qua internet tối đa 1 MB") {
		t.Fatalf("lời nhắn = %q, phải nêu trần", e.Error())
	}

	// Local: giữ câu tiếng Anh của API editor.
	t.Setenv("PUBLIC_URL", "")
	_, e = s.ImportMedia(ctx, ImportMediaOptions{FileName: "image.png", DeclaredMIME: "image/png", Body: bytes.NewReader(append(b, make([]byte, 2<<20)...))})
	mediaErrorCode(t, e, "payload_too_large")
	if strings.Contains(e.Error(), "Tải lên qua internet") {
		t.Fatalf("local không được dùng lời nhắn công khai: %q", e.Error())
	}
}
