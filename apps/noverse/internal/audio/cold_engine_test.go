package audio

import (
	"context"
	"testing"
	"time"
)

// coldSynth mô phỏng máy đọc TTS chưa bật: engine thật bật tiến trình Python và
// nạp mô hình 20–50 giây (tới 8 phút) trước khi trả danh sách giọng, nên ở đây
// Voices chỉ trả về khi ctx của người gọi hết hạn.
type coldSynth struct{ calls int }

func (*coldSynth) Capabilities() Capabilities    { return Capabilities{} }
func (*coldSynth) Status(context.Context) Status { return Status{Engine: "vieneu", State: "stopped"} }
func (s *coldSynth) Voices(ctx context.Context) ([]Voice, error) {
	s.calls++
	<-ctx.Done()
	return nil, ctx.Err()
}
func (*coldSynth) Synthesize(context.Context, SynthRequest) (Audio, error) {
	return Audio{}, ErrEngineMissing
}
func (*coldSynth) AddVoice(context.Context, string, string) (Voice, error) {
	return Voice{}, ErrEngineMissing
}

// TestListVoicesDoesNotWaitForColdEngine: danh sách giọng của màn đọc phải trả về
// trong trần voiceListTimeout dù máy đọc đang nạp mô hình. Trước khi sửa, nó chờ
// đúng ctx của request (không có hạn) nên màn Audio đứng ở skeleton 20–50 giây và
// nếu người dùng tải lại thì yêu cầu bị huỷ giữa lúc máy đọc đang bật.
func TestListVoicesDoesNotWaitForColdEngine(t *testing.T) {
	old := voiceListTimeout
	voiceListTimeout = 120 * time.Millisecond
	t.Cleanup(func() { voiceListTimeout = old })

	synth := &coldSynth{}
	layout := Layout{StoryDir: t.TempDir()}
	layout.AudioDir = layout.StoryDir + "/audio"
	layout.AssetsDir = layout.StoryDir + "/tai-san"
	layout.ExportsDir = layout.StoryDir + "/xuat-ban"

	done := make(chan struct{})
	var got []ProjectVoice
	var err error
	go func() {
		got, err = ListVoices(context.Background(), layout, synth)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("ListVoices chờ máy đọc quá 3 giây (trần %s)", voiceListTimeout)
	}
	if err != nil {
		t.Fatalf("ListVoices: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("không có giọng dự án và máy đọc chưa sẵn sàng → muốn rỗng, được %+v", got)
	}
	if synth.calls != 1 {
		t.Fatalf("hỏi máy đọc %d lần, muốn 1", synth.calls)
	}
}

// TestVolumesDoesNotWaitForColdEngine: cùng lý do ở màn tập audio — đây là request
// giữ skeleton của cả màn, nên nó phải xong dù máy đọc chưa lên. Thiếu preset chỉ
// mất tên giọng có sẵn, không đổi khoá "cần tạo lại" (VoiceID lấy từ phân vai).
func TestVolumesDoesNotWaitForColdEngine(t *testing.T) {
	old := voiceListTimeout
	voiceListTimeout = 120 * time.Millisecond
	t.Cleanup(func() { voiceListTimeout = old })

	layout := Layout{StoryDir: t.TempDir()}
	layout.AudioDir = layout.StoryDir + "/audio"
	layout.AssetsDir = layout.StoryDir + "/tai-san"
	layout.ExportsDir = layout.StoryDir + "/xuat-ban"
	synth := &coldSynth{}
	svc, err := NewService(layout, synth)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)

	voiceID := "Mai Anh"
	if _, err := svc.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &voiceID}}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteScript(layout, &Script{Chapter: 1, Volume: 1, Status: StatusReview,
		Segments: []Segment{{ID: "s1", Speaker: NarratorID, Text: "a"}}}); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	var view VolumeView
	go func() {
		view, err = svc.Volumes(context.Background(), []SourceChapter{{Chapter: 1, Title: "Một", Text: "một", Completed: true}}, 1)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("Volumes chờ máy đọc quá 3 giây (trần %s)", voiceListTimeout)
	}
	if err != nil {
		t.Fatalf("Volumes: %v", err)
	}
	if len(view.Chapters) != 1 || view.Chapters[0].Segments != 1 {
		t.Fatalf("view = %+v", view)
	}
}
