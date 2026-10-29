package stylestat

import (
	"reflect"
	"sort"
	"sync"
	"testing"
)

func TestTrackerMatchesComputeAcrossUpdates(t *testing.T) {
	allChapters := map[int]string{
		1: chapterWith("Đêm ấy, không phải do dự mà là sợ hãi. Đỉnh Thanh Vân gió thổi mạnh dần.\nCả đời này ta chưa từng đi xa, mong ngươi thay ta nhìn ngắm núi sông phương ấy.\nHắn bỏ đi."),
		2: chapterWith("Sáng sớm, nàng im lặng hồi lâu, mây trên đỉnh Thanh Vân cuộn lên.\nCả đời này ta chưa từng đi xa, mong ngươi thay ta nhìn ngắm núi sông phương ấy.\nTrời sáng dần."),
		3: chapterWith("Lục Cửu Uyên đứng trên đỉnh Thanh Vân, khoé môi nhếch lên.\nCả đời này ta chưa từng đi xa, mong ngươi thay ta nhìn ngắm núi sông phương ấy.\nKhông ai đáp."),
		4: chapterWith("Mọi người nhìn về đỉnh Thanh Vân, cảm thấy mưa sắp tới.\nCuối con phố dài vọng tiếng chuông.\nCửa mở."),
		5: chapterWith("Như thể một giấc mơ cũ đè nặng trên đỉnh Thanh Vân.\nMột cảm giác khó tả lan dọc bậc đá.\nĐèn tắt."),
		6: chapterWith("Tim hắn thắt lại, nhưng không quay đầu.\nĐỉnh Thanh Vân vẫn chìm trong mây.\nCâu chuyện vẫn phải đi tiếp về phía trước."),
	}
	titles := []string{"Chương 1: Gió nổi", "Mây dâng", "Chương 3: Sấm động", "Ngầm triều", "Đường về", "Cổng núi"}
	stopwords := []string{"Lục Cửu Uyên"}

	tracker := NewTracker()
	chapters := make(map[int]string)
	for chapter := 1; chapter <= 6; chapter++ {
		chapters[chapter] = allChapters[chapter]
		tracker.Upsert(chapter, allChapters[chapter])
		assertTrackerMatchesCompute(t, tracker, chapters, titles, stopwords)
	}

	chapters[2] = chapterWith("Rạng đông, tim nàng trĩu xuống, tựa như giấc mơ cũ bừng tỉnh.\nCâu dài viết lại chỉ xuất hiện ở chương này, không được thành lặp xuyên chương.\nGió ngừng.")
	tracker.Upsert(2, chapters[2])
	assertTrackerMatchesCompute(t, tracker, chapters, titles, stopwords)

	delete(chapters, 4)
	tracker.Remove(4)
	assertTrackerMatchesCompute(t, tracker, chapters, titles, stopwords)
}

func TestTrackerSnapshotReturnsIndependentCopy(t *testing.T) {
	tracker := NewTracker()
	for chapter := 1; chapter <= 5; chapter++ {
		tracker.Upsert(chapter, chapterWith("Không phải lùi bước mà là chờ đợi."))
	}

	first := tracker.Snapshot(nil, nil)
	if first == nil || len(first.Patterns) == 0 {
		t.Fatalf("unexpected first snapshot: %+v", first)
	}
	first.Patterns[0].Total = 999

	second := tracker.Snapshot(nil, nil)
	if second.Patterns[0].Total == 999 {
		t.Fatal("cached snapshot was mutated through caller result")
	}
}

func TestTrackerConcurrentSnapshotAndUpdate(t *testing.T) {
	tracker := NewTracker()
	for chapter := 1; chapter <= 8; chapter++ {
		tracker.Upsert(chapter, chapterWith("Không phải lùi bước mà là chờ đợi."))
	}

	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if worker%2 == 0 {
					tracker.Upsert(8, chapterWith("Không phải lùi bước mà là chờ đợi."))
				} else {
					_ = tracker.Snapshot([]string{"Chương 1"}, []string{"Lâm Nghiễn"})
				}
			}
		}(worker)
	}
	wg.Wait()
}

func assertTrackerMatchesCompute(
	t *testing.T,
	tracker *Tracker,
	chapters map[int]string,
	titles, stopwords []string,
) {
	t.Helper()
	ids := make([]int, 0, len(chapters))
	for chapter := range chapters {
		ids = append(ids, chapter)
	}
	sort.Ints(ids)
	texts := make([]string, 0, len(ids))
	for _, chapter := range ids {
		texts = append(texts, chapters[chapter])
	}

	want := Compute(Input{Chapters: texts, Titles: titles, Stopwords: stopwords})
	got := tracker.Snapshot(titles, stopwords)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tracker mismatch\n got: %+v\nwant: %+v", got, want)
	}
}
