package library

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

const probeJSON = `{"format":{"duration":"4.0"},"streams":[{"codec_type":"video","width":1280,"height":720,"duration":"4.0"}]}`

// mp4Head là chữ ký ISO BMFF tối thiểu (ftyp box) để qua bước sniff.
func mp4Head() []byte {
	return append([]byte{0, 0, 0, 24}, []byte("ftypisom")...)
}

// mp4Content là tệp giả có chữ ký mp4 hợp lệ + phần thân bất kỳ.
func mp4Content() []byte {
	return append(mp4Head(), bytes.Repeat([]byte{0xAB}, 64)...)
}

// pngHead là chữ ký PNG tối thiểu.
func pngHead() []byte {
	return append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte{0xCD}, 32)...)
}

// solidFrames dựng bgSampleCount khung rgb24 một màu.
func solidFrames(r, g, b byte) []byte {
	frame := make([]byte, bgSampleWidth*bgSampleHeight*3)
	for i := 0; i < len(frame); i += 3 {
		frame[i], frame[i+1], frame[i+2] = r, g, b
	}
	return bytes.Repeat(frame, bgSampleCount)
}

func TestClassifyBackground(t *testing.T) {
	if got := classifyBackground(solidFrames(0, 0, 0), bgSampleWidth, bgSampleHeight); got != domain.BgBlack {
		t.Fatalf("đen = %s", got)
	}
	if got := classifyBackground(solidFrames(10, 200, 20), bgSampleWidth, bgSampleHeight); got != domain.BgGreen {
		t.Fatalf("xanh = %s", got)
	}
	if got := classifyBackground(solidFrames(200, 30, 40), bgSampleWidth, bgSampleHeight); got != domain.BgNone {
		t.Fatalf("đỏ = %s", got)
	}
	if got := classifyBackground(nil, bgSampleWidth, bgSampleHeight); got != domain.BgNone {
		t.Fatalf("rỗng = %s", got)
	}
	// nửa đen nửa sáng: viền không đủ 85% tối → none
	mixed := solidFrames(0, 0, 0)
	copy(mixed[len(mixed)/2:], solidFrames(220, 220, 220)[:len(mixed)-len(mixed)/2])
	if got := classifyBackground(mixed, bgSampleWidth, bgSampleHeight); got == domain.BgBlack {
		t.Fatalf("nửa sáng vẫn bị coi là đen")
	}
}

func TestAddOverlayFileDetectsMetadataAndDedupes(t *testing.T) {
	store := newMemStore()
	runner := &fakeRunner{probe: probeJSON, frames: solidFrames(0, 0, 0)}
	svc := testService(t, store, func(d *Deps) {
		d.Runner = runner
		d.Bins = fakeBins{ok: true}
	})
	ctx := context.Background()
	content := mp4Content()

	overlay, reused, err := svc.AddOverlayReader(ctx, "pháo hoa.mp4", "", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	if reused {
		t.Fatal("lần đầu không phải dùng lại")
	}
	if overlay.Name != "pháo hoa" || overlay.Mime != "video/mp4" || overlay.Bytes != int64(len(content)) {
		t.Fatalf("overlay = %+v", overlay)
	}
	if overlay.DurationSec == nil || *overlay.DurationSec != 4 || overlay.Width == nil || *overlay.Width != 1280 {
		t.Fatalf("metadata = %+v", overlay)
	}
	if overlay.Background != domain.BgBlack {
		t.Fatalf("nền = %s", overlay.Background)
	}
	if !strings.HasPrefix(overlay.FilePath, OverlayDir+"/") || !strings.HasSuffix(overlay.FilePath, ".mp4") {
		t.Fatalf("đường dẫn = %q", overlay.FilePath)
	}
	path, err := svc.files.Resolve(overlay.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if onDisk, err := os.ReadFile(path); err != nil || !bytes.Equal(onDisk, content) {
		t.Fatalf("tệp trên đĩa = %q %v", string(onDisk), err)
	}

	again, reused, err := svc.AddOverlayReader(ctx, "ten-khac.mp4", "", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	if !reused || again.ID != overlay.ID {
		t.Fatalf("trùng byte = %+v reused=%v", again, reused)
	}
	list, _ := svc.ListOverlays(ctx)
	if len(list) != 1 {
		t.Fatalf("số dòng = %d", len(list))
	}
}

func TestListOverlaysNewestFirst(t *testing.T) {
	store := newMemStore()
	clock := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	svc := testService(t, store, func(d *Deps) {
		d.Runner = &fakeRunner{probe: probeJSON, frames: solidFrames(0, 0, 0)}
		d.Bins = fakeBins{ok: true}
		d.Now = func() time.Time {
			now := clock
			clock = clock.Add(time.Second)
			return now
		}
	})
	ctx := context.Background()
	first, _, err := svc.AddOverlayReader(ctx, "cu.mp4", "", bytes.NewReader(mp4Content()), int64(len(mp4Content())))
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := svc.AddOverlayReader(ctx, "moi.mp4", "", bytes.NewReader(append(mp4Content(), 0x01)), int64(len(mp4Content())+1))
	if err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListOverlays(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v %v", list, err)
	}
	if list[0].ID != second.ID || list[1].ID != first.ID {
		t.Fatalf("thứ tự = %+v", list)
	}
}

func TestAddOverlayRejectsBadInput(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store, func(d *Deps) {
		d.Runner = &fakeRunner{probe: probeJSON, frames: solidFrames(0, 0, 0)}
		d.Bins = fakeBins{ok: true}
	})
	ctx := context.Background()
	cases := []struct {
		name string
		file string
		body []byte
		size int64
		code string
	}{
		{"đuôi lạ", "note.txt", mp4Content(), 3, CodeInvalidInput},
		{"rỗng", "x.mp4", mp4Content(), 0, CodeBadMedia},
		{"quá lớn", "x.mp4", mp4Content(), MaxOverlayBytes + 1, CodeInvalidInput},
		{"mp4 đổi tên thành png", "x.png", mp4Content(), 3, CodeBadMedia},
		{"rác không có chữ ký", "x.mp4", []byte("abc"), 3, CodeBadMedia},
		{"png giả làm mp4", "x.mp4", pngHead(), 3, CodeBadMedia},
	}
	for _, tc := range cases {
		_, _, err := svc.AddOverlayReader(ctx, tc.file, "", bytes.NewReader(tc.body), tc.size)
		if CodeOf(err) != tc.code {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	if _, _, err := svc.AddOverlayReader(ctx, "x.png", "", bytes.NewReader(pngHead()), int64(len(pngHead()))); err != nil {
		t.Fatalf("ảnh png phải nhận: %v", err)
	}
	// Tên hiển thị tiếng Việt không đuôi không làm hỏng upload.
	uploaded, _, err := svc.AddOverlayReader(ctx, "x.png", "Mưa trên kính", bytes.NewReader(append(pngHead(), 0x02)), int64(len(pngHead())+1))
	if err != nil {
		t.Fatalf("tên hiển thị: %v", err)
	}
	if uploaded.Name != "Mưa trên kính" || uploaded.Mime != "image/png" {
		t.Fatalf("overlay = %+v", uploaded)
	}
}

func TestAddOverlayWithoutFfmpegIsToolMissing(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store, func(d *Deps) {
		d.Runner = &fakeRunner{probe: probeJSON}
		d.Bins = fakeBins{ok: false}
	})
	_, _, err := svc.AddOverlayReader(context.Background(), "x.mp4", "", bytes.NewReader(mp4Content()), int64(len(mp4Content())))
	if CodeOf(err) != CodeToolMissing {
		t.Fatalf("lỗi = %v", err)
	}
}

func TestRemoveOverlayTombstonesAndRestoresOnReAdd(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store, func(d *Deps) {
		d.Runner = &fakeRunner{probe: probeJSON, frames: solidFrames(0, 0, 0)}
		d.Bins = fakeBins{ok: true}
	})
	ctx := context.Background()
	content := mp4Content()
	overlay, _, err := svc.AddOverlayReader(ctx, "mua.mp4", "", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RemoveOverlay(ctx, overlay.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := svc.ListOverlays(ctx); len(list) != 0 {
		t.Fatalf("vẫn còn: %+v", list)
	}
	if _, err := svc.GetOverlay(ctx, overlay.ID); CodeOf(err) != CodeNotFound {
		t.Fatalf("đọc sau khi bỏ: %v", err)
	}
	if err := svc.RemoveOverlay(ctx, overlay.ID); CodeOf(err) != CodeNotFound {
		t.Fatalf("bỏ lần hai: %v", err)
	}
	restored, reused, err := svc.AddOverlayReader(ctx, "mua-2.mp4", "", bytes.NewReader(content), int64(len(content)))
	if err != nil || !reused || restored.ID != overlay.ID || restored.Name != "mua-2" {
		t.Fatalf("thêm lại = %+v reused=%v err=%v", restored, reused, err)
	}
	if list, _ := svc.ListOverlays(ctx); len(list) != 1 {
		t.Fatalf("khôi phục = %+v", list)
	}
}

func TestSeedBuiltinsIdempotentAndBuiltinsCannotBeRemoved(t *testing.T) {
	store := newMemStore()
	runner := &fakeRunner{probe: probeJSON, frames: solidFrames(0, 0, 0)}
	svc := testService(t, store, func(d *Deps) {
		d.Runner = runner
		d.Bins = fakeBins{ok: true}
	})
	ctx := context.Background()

	added, err := svc.SeedBuiltins(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if added != 12 {
		t.Fatalf("lần đầu thêm %d", added)
	}
	list, _ := svc.ListOverlays(ctx)
	if len(list) != 12 {
		t.Fatalf("số hiệu ứng = %d", len(list))
	}
	byBuiltin := make(map[string]domain.Overlay, len(list))
	for _, overlay := range list {
		if overlay.Builtin == "" {
			t.Fatalf("thiếu builtin: %+v", overlay)
		}
		byBuiltin[overlay.Builtin] = overlay
		if overlay.Mime != "video/mp4" || overlay.Background != domain.BgBlack || overlay.DurationSec == nil {
			t.Fatalf("metadata %s = %+v", overlay.Builtin, overlay)
		}
	}
	if byBuiltin["snow"].Name != "Tuyết" {
		t.Fatalf("tên tuyết = %q", byBuiltin["snow"].Name)
	}
	if _, err := os.Stat(mustResolve(t, svc, byBuiltin["rain"].FilePath)); err != nil {
		t.Fatalf("tệp mưa: %v", err)
	}

	added, err = svc.SeedBuiltins(ctx)
	if err != nil || added != 0 {
		t.Fatalf("lần hai thêm %d (%v)", added, err)
	}
	if err := svc.RemoveOverlay(ctx, byBuiltin["fog"].ID); CodeOf(err) != CodeBuiltin {
		t.Fatalf("bỏ hiệu ứng có sẵn: %v", err)
	}
	if list, _ := svc.ListOverlays(ctx); len(list) != 12 {
		t.Fatalf("số hiệu ứng = %d", len(list))
	}
}

// mustResolve đổi đường dẫn tương đối qua Files của service.
func mustResolve(t *testing.T, svc *Service, rel string) string {
	t.Helper()
	path, err := svc.files.Resolve(rel)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReAddRepairsMissingFile(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store, func(d *Deps) {
		d.Runner = &fakeRunner{probe: probeJSON, frames: solidFrames(0, 0, 0)}
		d.Bins = fakeBins{ok: true}
	})
	ctx := context.Background()
	content := mp4Content()
	overlay, _, err := svc.AddOverlayReader(ctx, "mua.mp4", "", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	target := mustResolve(t, svc, overlay.FilePath)
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.OverlayPath(ctx, overlay.ID); CodeOf(err) != CodeBadMedia {
		t.Fatalf("đọc sau khi mất tệp: %v", err)
	}
	// Tải lại đúng bytes: dùng lại dòng cũ và tự phục hồi tệp.
	again, reused, err := svc.AddOverlayReader(ctx, "mua.mp4", "", bytes.NewReader(content), int64(len(content)))
	if err != nil || !reused || again.ID != overlay.ID {
		t.Fatalf("tải lại = %+v reused=%v err=%v", again, reused, err)
	}
	if _, _, err := svc.OverlayPath(ctx, overlay.ID); err != nil {
		t.Fatalf("tệp chưa phục hồi: %v", err)
	}
	onDisk, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(onDisk, content) {
		t.Fatalf("nội dung phục hồi sai: %v", err)
	}
}

func TestSeedBuiltinsConcurrentKeepsFiles(t *testing.T) {
	store := newMemStore()
	files := tempFiles{root: t.TempDir()}
	deps := func(d *Deps) {
		d.Runner = &fakeRunner{probe: probeJSON, frames: solidFrames(0, 0, 0)}
		d.Bins = fakeBins{ok: true}
		d.Files = files
	}
	counter := 0
	var idMu sync.Mutex
	newID := func() string {
		idMu.Lock()
		defer idMu.Unlock()
		counter++
		return fmt.Sprintf("aaaaaaaa-0000-4000-8000-%012d", counter)
	}
	svcA := testService(t, store, deps, func(d *Deps) { d.NewID = newID })
	svcB := testService(t, store, deps, func(d *Deps) { d.NewID = newID })
	ctx := context.Background()

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-start; _, err := svcA.SeedBuiltins(ctx); results <- err }()
	go func() { <-start; _, err := svcB.SeedBuiltins(ctx); results <- err }()
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("seed đua: %v", err)
		}
	}
	list, err := svcA.ListOverlays(ctx)
	if err != nil || len(list) != 12 {
		t.Fatalf("số hiệu ứng = %d %v", len(list), err)
	}
	for _, overlay := range list {
		path := mustResolve(t, svcA, overlay.FilePath)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("tệp %s bị xoá sau đua: %v", overlay.FilePath, err)
		}
	}
}
