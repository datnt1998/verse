// Ca test cho xem trước Khám Phá: phân biệt artifact vắng (dò một lần) với
// artifact hỏng (lỗi thật, không gọi mạng), đệm kết quả dò theo (đối thủ,
// video), và không biến request bị huỷ thành lỗi nội dung.
package rivals

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

const (
	testPreviewVideo  = "vidid000001"
	testPreviewRival  = "UCrival00001"
	testPreviewPath   = "download/source.info.json"
	otherPreviewRival = "UCrival00002"
)

// previewFixture dựng Service trên fakeStore + deps ghi lại số lượt mở tệp và
// dò mạng để test khẳng định "không gọi mạng khi đã có artifact".
type previewFixture struct {
	store    *fakeStore
	svc      *Service
	opens    int
	probes   int
	opened   []string
	artifact map[string]string
	openErr  map[string]error
	probeRaw []byte
	probeErr error
}

func newPreviewFixture(t *testing.T) *previewFixture {
	t.Helper()
	f := &previewFixture{
		store: &fakeStore{
			videoChan: map[string]string{testPreviewVideo: testPreviewRival},
			infoPaths: map[string]string{},
		},
		artifact: map[string]string{},
		openErr:  map[string]error{},
		probeRaw: chaptersJSON(),
	}
	f.svc = NewWithPreview(f.store, PreviewDeps{
		OpenArtifact: func(ctx context.Context, path string) (io.ReadCloser, error) {
			f.opens++
			f.opened = append(f.opened, path)
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if err, ok := f.openErr[path]; ok {
				return nil, err
			}
			raw, ok := f.artifact[path]
			if !ok {
				return nil, ErrArtifactAbsent
			}
			return io.NopCloser(strings.NewReader(raw)), nil
		},
		ProbeInfo: func(ctx context.Context, _ string) ([]byte, error) {
			f.probes++
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if f.probeErr != nil {
				return nil, f.probeErr
			}
			return f.probeRaw, nil
		},
	})
	return f
}

func chaptersJSON(titles ...string) []byte {
	var b strings.Builder
	b.WriteString(`{"chapters":[`)
	for i, title := range titles {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"start_time":%d,"title":%q}`, (i+1)*10, title)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

func TestPreviewReadsPublishedArtifactEveryTimeWithoutProbe(t *testing.T) {
	f := newPreviewFixture(t)
	f.store.infoPaths[testPreviewVideo] = testPreviewPath
	f.artifact[testPreviewPath] = string(chaptersJSON("Mở đầu", "Cao trào"))

	for i := range 2 {
		got, err := f.svc.Preview(context.Background(), "", testPreviewVideo)
		if err != nil {
			t.Fatalf("lần %d: %v", i, err)
		}
		if got.Source != "download" || got.State != "ready" || got.VideoID != testPreviewVideo {
			t.Fatalf("lần %d preview = %+v", i, got)
		}
		if len(got.Chapters) != 2 || got.Chapters[0].Title != "Mở đầu" || got.Chapters[1].StartSec != 20 {
			t.Fatalf("lần %d chapters = %+v", i, got.Chapters)
		}
	}
	if f.probes != 0 {
		t.Fatalf("đã dò mạng %d lần dù có artifact", f.probes)
	}
	// Artifact là nguồn sự thật: mỗi lần mở đọc lại, không đệm thay nó.
	if f.opens != 2 {
		t.Fatalf("số lượt mở artifact = %d, muốn 2", f.opens)
	}
}

func TestPreviewProbesOnceForAbsentArtifactThenCaches(t *testing.T) {
	f := newPreviewFixture(t)
	base := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	f.svc.now = func() time.Time { return base }
	f.probeRaw = chaptersJSON("Từ YouTube")

	first, err := f.svc.Preview(context.Background(), "", testPreviewVideo)
	if err != nil {
		t.Fatal(err)
	}
	if first.Source != "youtube" || len(first.Chapters) != 1 {
		t.Fatalf("preview = %+v", first)
	}
	// Sửa kết quả trả ra không được làm hỏng bản đệm.
	first.Chapters[0].Title = "bị sửa"

	second, err := f.svc.Preview(context.Background(), "", testPreviewVideo)
	if err != nil {
		t.Fatal(err)
	}
	if f.probes != 1 {
		t.Fatalf("dò mạng %d lần trong hạn đệm, muốn 1", f.probes)
	}
	if second.Chapters[0].Title != "Từ YouTube" {
		t.Fatalf("bản đệm bị lộ tham chiếu: %+v", second.Chapters)
	}

	// Hết hạn đệm thì dò lại một lần.
	base = base.Add(previewProbeTTL + time.Second)
	if _, err := f.svc.Preview(context.Background(), "", testPreviewVideo); err != nil {
		t.Fatal(err)
	}
	if f.probes != 2 {
		t.Fatalf("sau TTL số lượt dò = %d, muốn 2", f.probes)
	}
}

func TestPreviewCachesEmptyChaptersAsValid(t *testing.T) {
	f := newPreviewFixture(t)
	f.probeRaw = []byte(`{}`)

	for i := range 2 {
		got, err := f.svc.Preview(context.Background(), "", testPreviewVideo)
		if err != nil {
			t.Fatalf("lần %d: %v", i, err)
		}
		if got.Source != "youtube" || got.State != "empty" || len(got.Chapters) != 0 {
			t.Fatalf("lần %d preview = %+v", i, got)
		}
	}
	if f.probes != 1 {
		t.Fatalf("video không có chương bị dò %d lần, muốn 1", f.probes)
	}
}

func TestPreviewMissingBlobCountsAsAbsentAndProbes(t *testing.T) {
	f := newPreviewFixture(t)
	f.store.infoPaths[testPreviewVideo] = testPreviewPath
	f.openErr[testPreviewPath] = fmt.Errorf("%w: blob đã bị dọn", ErrArtifactAbsent)
	f.probeRaw = chaptersJSON("Từ YouTube")

	got, err := f.svc.Preview(context.Background(), "", testPreviewVideo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "youtube" || f.probes != 1 {
		t.Fatalf("preview = %+v, probes = %d", got, f.probes)
	}
}

func TestPreviewCorruptArtifactIsActionableAndNeverProbes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want error
	}{
		{name: "json hỏng", raw: `{"chapters": [`},
		{name: "quá lớn", raw: strings.Repeat("x", maxSourceInfoBytes+1), want: errSourceInfoTooLarge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newPreviewFixture(t)
			f.store.infoPaths[testPreviewVideo] = testPreviewPath
			f.artifact[testPreviewPath] = c.raw

			got, err := f.svc.Preview(context.Background(), "", testPreviewVideo)
			if err == nil || got != nil {
				t.Fatalf("preview = %+v, %v; muốn lỗi thật", got, err)
			}
			var pe *PreviewError
			if !errors.As(err, &pe) {
				t.Fatalf("err = %T %v, muốn *PreviewError", err, err)
			}
			if pe.Code != codeSourceUnreadable || pe.Message == "" || pe.Recovery == "" {
				t.Fatalf("PreviewError = %+v", pe)
			}
			if pe.Cause == nil {
				t.Fatal("PreviewError phải giữ lỗi đọc gốc")
			}
			if c.want != nil && !errors.Is(err, c.want) {
				t.Fatalf("errors.Is(%v) = false", c.want)
			}
			if f.probes != 0 {
				t.Fatalf("artifact hỏng nhưng vẫn dò mạng %d lần", f.probes)
			}
		})
	}
}

func TestPreviewArtifactReadErrorIsTypedNotNetworkFallback(t *testing.T) {
	f := newPreviewFixture(t)
	f.store.infoPaths[testPreviewVideo] = testPreviewPath
	f.openErr[testPreviewPath] = errors.New("permission denied")

	_, err := f.svc.Preview(context.Background(), "", testPreviewVideo)
	var pe *PreviewError
	if !errors.As(err, &pe) || pe.Code != codeSourceUnreadable {
		t.Fatalf("err = %v, muốn PreviewError %s", err, codeSourceUnreadable)
	}
	if f.probes != 0 {
		t.Fatalf("lỗi đọc artifact nhưng vẫn dò mạng %d lần", f.probes)
	}
}

func TestPreviewProbeFailureIsActionableAndRetried(t *testing.T) {
	f := newPreviewFixture(t)
	f.probeErr = errors.New("mạng không ổn định")

	for i := range 2 {
		_, err := f.svc.Preview(context.Background(), "", testPreviewVideo)
		var pe *PreviewError
		if !errors.As(err, &pe) || pe.Code != codePreviewUnavailable {
			t.Fatalf("lần %d err = %v, muốn PreviewError %s", i, err, codePreviewUnavailable)
		}
		if pe.Cause == nil {
			t.Fatalf("lần %d thiếu lỗi gốc", i)
		}
	}
	// Lỗi không được đệm: lượt sau vẫn thử lại.
	if f.probes != 2 {
		t.Fatalf("số lượt dò = %d, muốn 2 (lỗi không đệm)", f.probes)
	}

	f.probeErr = nil
	if got, err := f.svc.Preview(context.Background(), "", testPreviewVideo); err != nil || got.Source != "youtube" {
		t.Fatalf("sau khi mạng lại = %+v, %v", got, err)
	}
}

func TestPreviewProbeMalformedJSONIsActionable(t *testing.T) {
	f := newPreviewFixture(t)
	f.probeRaw = []byte(`<html>not json</html>`)

	_, err := f.svc.Preview(context.Background(), "", testPreviewVideo)
	var pe *PreviewError
	if !errors.As(err, &pe) || pe.Code != codePreviewUnavailable || pe.Cause == nil {
		t.Fatalf("err = %v, muốn PreviewError %s kèm lỗi gốc", err, codePreviewUnavailable)
	}
	if f.probes != 1 {
		t.Fatalf("probes = %d", f.probes)
	}
}

func TestPreviewCancellationReturnsContextError(t *testing.T) {
	t.Run("artifact", func(t *testing.T) {
		f := newPreviewFixture(t)
		f.store.infoPaths[testPreviewVideo] = testPreviewPath
		f.artifact[testPreviewPath] = string(chaptersJSON("Một"))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := f.svc.Preview(ctx, "", testPreviewVideo)
		var pe *PreviewError
		if errors.As(err, &pe) {
			t.Fatalf("huỷ request trả PreviewError %+v; muốn context.Canceled", pe)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, muốn context.Canceled", err)
		}
	})

	t.Run("probe", func(t *testing.T) {
		f := newPreviewFixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := f.svc.Preview(ctx, "", testPreviewVideo)
		var pe *PreviewError
		if errors.As(err, &pe) {
			t.Fatalf("huỷ request trả PreviewError %+v; muốn context.Canceled", pe)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, muốn context.Canceled", err)
		}
		// Lượt bị huỷ không được đệm kết quả dở dang.
		if _, err := f.svc.Preview(context.Background(), "", testPreviewVideo); err != nil {
			t.Fatalf("lượt sau: %v", err)
		}
		if f.probes != 2 {
			t.Fatalf("probes = %d, muốn 2", f.probes)
		}
	})
}

func TestPreviewCacheIsScopedToRival(t *testing.T) {
	f := newPreviewFixture(t)
	f.probeRaw = chaptersJSON("Của kênh A")
	if _, err := f.svc.Preview(context.Background(), domain.AllRivals, testPreviewVideo); err != nil {
		t.Fatal(err)
	}

	// Cùng mã video nhưng dữ liệu quét lại gán cho đối thủ khác: phải dò lại,
	// không trả bản đệm của kênh trước.
	f.store.videoChan[testPreviewVideo] = otherPreviewRival
	f.probeRaw = chaptersJSON("Của kênh B")
	got, err := f.svc.Preview(context.Background(), otherPreviewRival, testPreviewVideo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Chapters) != 1 || got.Chapters[0].Title != "Của kênh B" {
		t.Fatalf("lộ đệm chéo kênh: %+v", got.Chapters)
	}
	if f.probes != 2 {
		t.Fatalf("probes = %d, muốn 2", f.probes)
	}
}

func TestPreviewRejectsOtherRivalWithoutReadingSource(t *testing.T) {
	f := newPreviewFixture(t)
	f.store.infoPaths[testPreviewVideo] = testPreviewPath
	f.artifact[testPreviewPath] = string(chaptersJSON("Một"))

	if _, err := f.svc.Preview(context.Background(), otherPreviewRival, testPreviewVideo); !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("err = %v, muốn ErrVideoNotFound", err)
	}
	if f.opens != 0 || f.probes != 0 {
		t.Fatalf("opens = %d, probes = %d; muốn 0", f.opens, f.probes)
	}
}

func TestPreviewInvalidVideoIDSkipsEverything(t *testing.T) {
	f := newPreviewFixture(t)
	if _, err := f.svc.Preview(context.Background(), "", "quá-ngắn"); !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("err = %v, muốn ErrVideoNotFound", err)
	}
	if f.opens != 0 || f.probes != 0 {
		t.Fatalf("opens = %d, probes = %d; muốn 0", f.opens, f.probes)
	}
}

func TestParseChaptersSortsSkipsAndBounds(t *testing.T) {
	raw := `{"chapters":[
		{"start_time":30,"title":"Ba"},
		{"title":"thiếu giờ"},
		{"start_time":10,"title":"  Một  "},
		{"start_time":-5,"title":"âm"},
		{"start_time":20,"title":"   "},
		{"start_time":20,"title":"Hai"}
	]}`
	got, err := parseChapters(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("chapters = %+v", got)
	}
	if got[0].Title != "Một" || got[1].Title != "Hai" || got[2].Title != "Ba" {
		t.Fatalf("thứ tự/trim sai: %+v", got)
	}

	var many strings.Builder
	many.WriteString(`{"chapters":[`)
	for i := range maxChapters + 50 {
		if i > 0 {
			many.WriteString(",")
		}
		fmt.Fprintf(&many, `{"start_time":%d,"title":"c%d"}`, i, i)
	}
	many.WriteString(`]}`)
	bounded, err := parseChapters(strings.NewReader(many.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(bounded) != maxChapters {
		t.Fatalf("len = %d, muốn %d", len(bounded), maxChapters)
	}

	empty, err := parseChapters(strings.NewReader(`{"chapters":null}`))
	if err != nil || len(empty) != 0 {
		t.Fatalf("chapters null = %+v, %v", empty, err)
	}
}
