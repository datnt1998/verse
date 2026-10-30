package rivals

import (
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func loc(t *testing.T, zone string) *time.Location {
	t.Helper()
	l, err := ZoneLocation(zone)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// at dựng một thời điểm UTC.
func at(y int, mo time.Month, d, h, mi int) time.Time {
	return time.Date(y, mo, d, h, mi, 0, 0, time.UTC)
}

// weeklyTimes dựng n lần đăng cách nhau đúng một tuần, cùng thứ/giờ theo UTC,
// kết thúc ở end.
func weeklyTimes(end time.Time, hourUTC, n int) []time.Time {
	base := time.Date(end.Year(), end.Month(), end.Day(), hourUTC, 0, 0, 0, time.UTC)
	out := make([]time.Time, 0, n)
	for i := range n {
		out = append(out, base.Add(-time.Duration(i)*7*24*time.Hour))
	}
	return out
}

func TestPostingSlotsEmptySample(t *testing.T) {
	s := PostingSlots("UCx", domain.ZoneVN, nil, 0, loc(t, "vn"))
	if s.Total != 0 || s.WeeksTotal != 0 || len(s.Slots) != 0 || s.OffSlot != 0 || s.PerWeek != nil {
		t.Fatalf("mẫu rỗng = %+v", s)
	}
}

func TestPostingSlotsUnderTwoUploads(t *testing.T) {
	s := PostingSlots("UCx", domain.ZoneVN, []time.Time{at(2026, 10, 5, 12, 0)}, 29, loc(t, "vn"))
	if s.Total != 1 || s.Pending != 29 || s.WeeksTotal != 1 {
		t.Fatalf("một video = %+v", s)
	}
	if s.PerWeek != nil {
		t.Fatalf("perWeek phải nil khi < 2 video, được %v", *s.PerWeek)
	}
	if len(s.Slots) != 0 {
		t.Fatalf("một video không tạo khung: %+v", s.Slots)
	}
	// Video nằm trong mẫu nhưng không đủ ngưỡng vẫn tô lên lưới…
	var total int
	for wd := range 7 {
		for b := range 12 {
			total += s.Grid[wd][b]
		}
	}
	if total != 1 {
		t.Fatalf("grid = %d", total)
	}
	// …và được tính là lẻ ngoài khung.
	if s.OffSlot != 1 {
		t.Fatalf("offSlot = %d", s.OffSlot)
	}
}

func TestPostingSlotsWeeklyRhythm(t *testing.T) {
	// 6 video, mỗi thứ Hai 14:00 UTC = 21:00 VN, 6 tuần liền.
	times := weeklyTimes(at(2026, 10, 26, 14, 0), 14, 6)
	s := PostingSlots("UCx", domain.ZoneVN, times, 0, loc(t, "vn"))
	if len(s.Slots) != 1 {
		t.Fatalf("slots = %+v", s.Slots)
	}
	got := s.Slots[0]
	if got.Rhythm != domain.RhythmWeekly || got.WeeksHit != 6 || got.WeeksTotal != 6 {
		t.Fatalf("rhythm = %+v", got)
	}
	if got.Count != 6 || !got.IsTop || len(got.Weekdays) != 1 || got.Weekdays[0] != 0 {
		t.Fatalf("slot = %+v", got)
	}
	if got.Start != "21:00" || got.End != "22:00" {
		t.Fatalf("khung = %s–%s", got.Start, got.End)
	}
	if s.OffSlot != 0 {
		t.Fatalf("offSlot = %d", s.OffSlot)
	}
	if s.PerWeek == nil || *s.PerWeek != 1 {
		t.Fatalf("perWeek = %v", s.PerWeek)
	}
}

func TestPostingSlotsMostWeeksRhythm(t *testing.T) {
	// 5 trên 6 tuần (≥ 75% nhưng không đủ) → most_weeks. Mẫu trải 6 tuần.
	times := weeklyTimes(at(2026, 10, 26, 14, 0), 14, 5)
	// Video lẻ ở tuần thứ 6 (trước loạt hit) để weeksTotal = 6.
	times = append(times, at(2026, 9, 23, 9, 0))
	s := PostingSlots("UCx", domain.ZoneVN, times, 0, loc(t, "vn"))
	var slot *domain.PostingSlot
	for i := range s.Slots {
		if s.Slots[i].Weekdays[0] == 0 {
			slot = &s.Slots[i]
		}
	}
	if slot == nil || slot.Rhythm != domain.RhythmMostWeeks || slot.WeeksHit != 5 {
		t.Fatalf("slots = %+v", s.Slots)
	}
}

func TestPostingSlotsNewRhythm(t *testing.T) {
	// 3 video chỉ trong 2 tuần gần nhất của mẫu dài 6 tuần → new; sinceWeeks = 2.
	times := []time.Time{
		at(2026, 10, 26, 14, 0), at(2026, 10, 19, 14, 0), at(2026, 10, 19, 15, 0),
		// Nền rải ở các tuần cũ để kéo weeksTotal = 6.
		at(2026, 9, 28, 9, 0), at(2026, 9, 14, 9, 0),
	}
	s := PostingSlots("UCx", domain.ZoneVN, times, 0, loc(t, "vn"))
	if len(s.Slots) != 1 {
		t.Fatalf("slots = %+v", s.Slots)
	}
	got := s.Slots[0]
	if got.Rhythm != domain.RhythmNew || got.SinceWeeks != 2 {
		t.Fatalf("slot = %+v (weeksTotal %d)", got, s.WeeksTotal)
	}
}

func TestPostingSlotsBiweeklyRhythm(t *testing.T) {
	// Đều hai tuần một lần: 3 lần đăng cách 14 ngày trên mẫu ~5 tuần →
	// 3/5 = 60% < 75%, không mới → biweekly.
	times := []time.Time{
		at(2026, 10, 26, 14, 0), at(2026, 10, 12, 14, 0), at(2026, 9, 28, 14, 0),
		at(2026, 10, 20, 9, 0), // video lẻ kéo tuần giữa
	}
	s := PostingSlots("UCx", domain.ZoneVN, times, 0, loc(t, "vn"))
	if len(s.Slots) != 1 || s.Slots[0].Rhythm != domain.RhythmBiweekly {
		t.Fatalf("slots = %+v", s.Slots)
	}
	if s.Slots[0].WeeksHit != 3 || s.WeeksTotal != 5 {
		t.Fatalf("slot = %+v weeksTotal %d", s.Slots[0], s.WeeksTotal)
	}
}

func TestPostingSlotsThreshold(t *testing.T) {
	// ≥ 3 video trên ≥ 2 tuần: đủ 3 video nhưng một tuần → không khung.
	single := []time.Time{at(2026, 10, 19, 14, 0), at(2026, 10, 19, 14, 30), at(2026, 10, 20, 14, 0), at(2026, 9, 28, 9, 0)}
	s := PostingSlots("UCx", domain.ZoneVN, single, 0, loc(t, "vn"))
	for _, slot := range s.Slots {
		if slot.Count == 3 && slot.WeeksHit == 1 {
			t.Fatalf("khung một tuần không được lặp: %+v", slot)
		}
	}
	// 2 video trên 2 tuần → không khung.
	two := []time.Time{at(2026, 10, 19, 14, 0), at(2026, 10, 12, 14, 0)}
	s2 := PostingSlots("UCx", domain.ZoneVN, two, 0, loc(t, "vn"))
	if len(s2.Slots) != 0 || s2.OffSlot != 2 {
		t.Fatalf("2 video 2 tuần = slots %+v off %d", s2.Slots, s2.OffSlot)
	}
}

func TestPostingSlotsAdjacentHoursMerge(t *testing.T) {
	// 19:00 và 20:00 liền nhau cùng thứ, ≥ 3 video trên ≥ 2 tuần → một khung 19–21.
	// 19:00 VN = 12:00 UTC; 20:00 VN = 13:00 UTC — cùng thứ Hai.
	times := []time.Time{
		at(2026, 10, 19, 12, 0), at(2026, 10, 12, 13, 0), at(2026, 10, 5, 12, 30),
	}
	s := PostingSlots("UCx", domain.ZoneVN, times, 0, loc(t, "vn"))
	if len(s.Slots) != 1 {
		t.Fatalf("slots = %+v", s.Slots)
	}
	got := s.Slots[0]
	if got.Start != "19:00" || got.End != "21:00" || got.Count != 3 {
		t.Fatalf("khung gộp = %+v", got)
	}
}

func TestPostingSlotsWeekdayMerge(t *testing.T) {
	// T2 và T4 cùng khung 21:00–22:00 VN, cùng đều → gộp một slot hai thứ;
	// weeksHit là hợp của các tuần, không cộng dồn.
	times := []time.Time{
		at(2026, 10, 5, 14, 0), at(2026, 10, 12, 14, 0), at(2026, 10, 19, 14, 0), // T2
		at(2026, 10, 7, 14, 0), at(2026, 10, 14, 14, 0), at(2026, 10, 21, 14, 0), // T4 cùng các tuần
	}
	s := PostingSlots("UCx", domain.ZoneVN, times, 0, loc(t, "vn"))
	if len(s.Slots) != 1 {
		t.Fatalf("slots = %+v", s.Slots)
	}
	got := s.Slots[0]
	if len(got.Weekdays) != 2 || got.Weekdays[0] != 0 || got.Weekdays[1] != 2 {
		t.Fatalf("weekdays = %+v", got.Weekdays)
	}
	if got.Count != 6 || got.WeeksHit != 3 || got.Rhythm != domain.RhythmWeekly {
		t.Fatalf("slot = %+v", got)
	}
}
func TestPostingSlotsIsTopSortsByCount(t *testing.T) {
	// Hai khung: khung nhiều video hơn đứng đầu và là isTop duy nhất.
	times := []time.Time{
		at(2026, 10, 26, 14, 0), at(2026, 10, 19, 14, 0), at(2026, 10, 12, 14, 0), at(2026, 10, 5, 14, 0),
		at(2026, 10, 27, 9, 0), at(2026, 10, 20, 9, 0), at(2026, 10, 13, 9, 0),
	}
	s := PostingSlots("UCx", domain.ZoneVN, times, 0, loc(t, "vn"))
	if len(s.Slots) != 2 {
		t.Fatalf("slots = %+v", s.Slots)
	}
	if !s.Slots[0].IsTop || s.Slots[1].IsTop {
		t.Fatalf("isTop = %+v", s.Slots)
	}
	if s.Slots[0].Count != 4 || s.Slots[1].Count != 3 {
		t.Fatalf("thứ tự = %+v", s.Slots)
	}
}

func TestPostingSlotsGrid(t *testing.T) {
	// Một video 21:xx VN thứ Hai → grid[0][10] (khối 20–22 giờ); video 10:30
	// VN thứ Hai → grid[0][5].
	times := []time.Time{at(2026, 10, 26, 14, 30), at(2026, 10, 19, 3, 30)}
	s := PostingSlots("UCx", domain.ZoneVN, times, 0, loc(t, "vn"))
	if s.Grid[0][10] != 1 {
		t.Fatalf("grid[0][10] = %d", s.Grid[0][10])
	}
	if s.Grid[0][5] != 1 {
		t.Fatalf("grid[0][5] = %d", s.Grid[0][5])
	}
}

func TestPostingSlotsZoneConversion(t *testing.T) {
	// Cùng một lần đăng: 21:30 VN thứ Hai = 10:30 New York thứ Hai (EDT,
	// VN − 11h). DST New York đổi offset theo mùa.
	times := []time.Time{at(2026, 10, 26, 14, 30), at(2026, 10, 19, 14, 30)}
	ny := PostingSlots("UCx", domain.ZoneNY, times, 0, loc(t, "ny"))
	if ny.Grid[0][5] != 2 {
		t.Fatalf("NY grid[0][5] = %d (muốn 2, 10:xx thứ Hai)", ny.Grid[0][5])
	}
	// Rollover qua nửa đêm: 00:30 VN thứ Tư = 13:30 T3 New York.
	roll := PostingSlots("UCx", domain.ZoneNY,
		[]time.Time{at(2026, 10, 27, 17, 30), at(2026, 10, 20, 17, 30)}, 0, loc(t, "ny"))
	if roll.Grid[1][6] != 2 {
		t.Fatalf("rollover: NY grid[1][6] = %d", roll.Grid[1][6])
	}
	vn := PostingSlots("UCx", domain.ZoneVN,
		[]time.Time{at(2026, 10, 27, 17, 30), at(2026, 10, 20, 17, 30)}, 0, loc(t, "vn"))
	if vn.Grid[2][0] != 2 {
		t.Fatalf("rollover: VN grid[2][0] = %d", vn.Grid[2][0])
	}
	// New York qua ranh DST: 2026-11-01 02:00 EDT → EST; cùng giờ UTC nhưng
	// mùa đông lệch thêm 1 giờ so với VN (−12h thay −11h).
	summer := PostingSlots("UCx", domain.ZoneNY, []time.Time{at(2026, 7, 6, 14, 0), at(2026, 7, 13, 14, 0)}, 0, loc(t, "ny"))
	if summer.Grid[0][5] != 2 { // 10:00 EDT thứ Hai
		t.Fatalf("EDT grid[0][5] = %d", summer.Grid[0][5])
	}
	winter := PostingSlots("UCx", domain.ZoneNY, []time.Time{at(2026, 12, 7, 14, 0), at(2026, 12, 14, 14, 0)}, 0, loc(t, "ny"))
	if winter.Grid[0][4] != 2 { // 09:00 EST thứ Hai
		t.Fatalf("EST grid[0][4] = %d", winter.Grid[0][4])
	}
}

func TestPostingSlotsPerWeek(t *testing.T) {
	times := weeklyTimes(at(2026, 10, 26, 14, 0), 14, 6)
	s := PostingSlots("UCx", domain.ZoneVN, times, 0, loc(t, "vn"))
	if s.PerWeek == nil || *s.PerWeek != 1 {
		t.Fatalf("perWeek = %v", s.PerWeek)
	}
}

func TestZoneLocationRejectsUnknown(t *testing.T) {
	if _, err := ZoneLocation("london"); err == nil {
		t.Fatal("múi giờ lạ phải lỗi")
	}
	if _, err := ZoneLocation(domain.ZoneVN); err != nil {
		t.Fatal(err)
	}
	if _, err := ZoneLocation(domain.ZoneNY); err != nil {
		t.Fatal(err)
	}
}

// TestPostingSlotsRhythmAtExactThreshold: đúng 75% số tuần là most_weeks,
// không phải new/biweekly (4 tuần mẫu, 3 tuần có mặt = 0,75).
func TestPostingSlotsRhythmAtExactThreshold(t *testing.T) {
	times := []time.Time{
		// Ba tuần liền có hit (tuần cuối, -1, -2) + một video nền ở tuần -3.
		at(2026, 10, 26, 14, 0), at(2026, 10, 19, 14, 0), at(2026, 10, 12, 14, 0),
		at(2026, 10, 5, 9, 0),
	}
	s := PostingSlots("UCx", domain.ZoneVN, times, 0, loc(t, "vn"))
	if s.WeeksTotal != 4 {
		t.Fatalf("weeksTotal = %d", s.WeeksTotal)
	}
	var slot *domain.PostingSlot
	for i := range s.Slots {
		if s.Slots[i].Weekdays[0] == 0 {
			slot = &s.Slots[i]
		}
	}
	if slot == nil {
		t.Fatalf("slots = %+v", s.Slots)
	}
	if slot.WeeksHit != 3 || slot.Rhythm != domain.RhythmMostWeeks {
		t.Fatalf("đúng 75%% phải là most_weeks: %+v", *slot)
	}
}

// TestPostingSlotsSameCountTieOrdering: cùng count thì thứ nhỏ hơn đứng
// trước, rồi tới giờ bắt đầu.
func TestPostingSlotsSameCountTieOrdering(t *testing.T) {
	times := []time.Time{
		// Thứ Năm 22:00 VN (15:00 UTC) và thứ Ba 07:00 VN (00:00 UTC), mỗi
		// khung 3 video trên 2 tuần, cùng count.
		at(2026, 10, 22, 15, 0), at(2026, 10, 15, 15, 0), at(2026, 10, 8, 15, 0),
		at(2026, 10, 20, 0, 0), at(2026, 10, 13, 0, 0), at(2026, 10, 6, 0, 0),
	}
	s := PostingSlots("UCx", domain.ZoneVN, times, 0, loc(t, "vn"))
	if len(s.Slots) != 2 {
		t.Fatalf("slots = %+v", s.Slots)
	}
	if s.Slots[0].Count != s.Slots[1].Count {
		t.Fatalf("mẫu không cùng count: %+v", s.Slots)
	}
	// Thứ Ba (1) trước thứ Năm (3) vì cùng count.
	if s.Slots[0].Weekdays[0] != 1 || s.Slots[1].Weekdays[0] != 3 {
		t.Fatalf("thứ tự tie = %+v", s.Slots)
	}
	if !s.Slots[0].IsTop || s.Slots[1].IsTop {
		t.Fatalf("isTop = %+v", s.Slots)
	}
}

// TestPostingSlotsDisjointWeekMerge: hai thứ cùng khung, cùng độ đều nhưng
// tập tuần khác nhau vẫn gộp; weeksHit là HỢP của hai tập (4), không phải
// tổng (6).
func TestPostingSlotsDisjointWeekMerge(t *testing.T) {
	times := []time.Time{
		// T2: tuần -6..-4 (3 tuần, rời hẳn tập của T6).
		at(2026, 9, 7, 14, 0), at(2026, 9, 14, 14, 0), at(2026, 9, 21, 14, 0),
		// T6: tuần -3..0 (4 tuần).
		at(2026, 10, 2, 14, 0), at(2026, 10, 9, 14, 0), at(2026, 10, 16, 14, 0), at(2026, 10, 23, 14, 0),
	}
	s := PostingSlots("UCx", domain.ZoneVN, times, 0, loc(t, "vn"))
	if len(s.Slots) != 1 {
		t.Fatalf("slots = %+v", s.Slots)
	}
	got := s.Slots[0]
	if got.Count != 7 || len(got.Weekdays) != 2 || got.Weekdays[0] != 0 || got.Weekdays[1] != 4 {
		t.Fatalf("gộp thứ = %+v", got)
	}
	if got.WeeksHit != 7 || got.WeeksTotal != 7 {
		t.Fatalf("weeksHit = %d/%d, muốn hợp 7 (=3+4) không phải tổng sai", got.WeeksHit, got.WeeksTotal)
	}
	if got.Rhythm != domain.RhythmBiweekly {
		t.Fatalf("rhythm = %s", got.Rhythm)
	}
}

// TestPostingSlotsMidnightBoundaries: đúng 00:00 và 23:59 giữ đúng thứ và
// khối lưới; khung 23:00–00:00 không gộp vòng sang thứ kế.
func TestPostingSlotsMidnightBoundaries(t *testing.T) {
	// 00:00 VN thứ Hai = 17:00 UTC Chủ nhật.
	midnight := []time.Time{at(2026, 10, 25, 17, 0), at(2026, 10, 18, 17, 0), at(2026, 10, 11, 17, 0)}
	s := PostingSlots("UCx", domain.ZoneVN, midnight, 0, loc(t, "vn"))
	if s.Grid[0][0] != 3 {
		t.Fatalf("00:00 thứ Hai: grid[0][0] = %d", s.Grid[0][0])
	}
	if len(s.Slots) != 1 || s.Slots[0].Start != "00:00" || s.Slots[0].End != "01:00" {
		t.Fatalf("khung 00:00 = %+v", s.Slots)
	}
	// 23:59 VN thứ Hai = 16:59 UTC thứ Hai.
	late := []time.Time{at(2026, 10, 26, 16, 59), at(2026, 10, 19, 16, 59), at(2026, 10, 12, 16, 59)}
	s = PostingSlots("UCx", domain.ZoneVN, late, 0, loc(t, "vn"))
	if s.Grid[0][11] != 3 {
		t.Fatalf("23:59 thứ Hai: grid[0][11] = %d", s.Grid[0][11])
	}
	if len(s.Slots) != 1 || s.Slots[0].Start != "23:00" || s.Slots[0].End != "00:00" {
		t.Fatalf("khung 23:00 = %+v", s.Slots)
	}
	if s.Slots[0].Weekdays[0] != 0 {
		t.Fatalf("khung 23:00 phải ở thứ Hai: %+v", s.Slots[0])
	}
}

// TestPostingSlotsNewYorkFallBackHour: giờ 01:30 lặp lại khi New York lùi
// giờ vẫn là một khung 01:00–02:00 Chủ nhật trên hai tuần khác nhau.
func TestPostingSlotsNewYorkFallBackHour(t *testing.T) {
	// 2026-11-01 01:30 EDT (05:30 UTC) và 01:30 EST (06:30 UTC) cùng ngày,
	// cộng 2026-10-25 01:30 EDT (05:30 UTC) — hai tuần, ba lần đăng.
	times := []time.Time{at(2026, 10, 25, 5, 30), at(2026, 11, 1, 5, 30), at(2026, 11, 1, 6, 30)}
	s := PostingSlots("UCx", domain.ZoneNY, times, 0, loc(t, "ny"))
	if len(s.Slots) != 1 {
		t.Fatalf("slots = %+v", s.Slots)
	}
	got := s.Slots[0]
	if got.Start != "01:00" || got.End != "02:00" || got.Count != 3 || got.WeeksHit != 2 {
		t.Fatalf("giờ lặp fall-back = %+v", got)
	}
	if got.Weekdays[0] != 6 {
		t.Fatalf("phải là Chủ nhật (6), được %d", got.Weekdays[0])
	}
}

// TestPostingSlotsSampleMixBeyondThirty: mẫu > 30 video trộn giờ chính xác và
// chưa đọc vẫn ra total/pending đúng và khung chỉ tính video có giờ.
func TestPostingSlotsSampleMixBeyondThirty(t *testing.T) {
	times := []time.Time{}
	for w := range 6 {
		times = append(times, at(2026, 10, 26, 14, 0).Add(-time.Duration(w)*7*24*time.Hour))
	}
	// 31 video: 6 có giờ, 25 pending (chưa đọc) — pending không bị tính vào
	// khung hay total.
	s := PostingSlots("UCx", domain.ZoneVN, times, 25, loc(t, "vn"))
	if s.Total != 6 || s.Pending != 25 || s.Sample != domain.PostingSample {
		t.Fatalf("total/pending = %d/%d (sample %d)", s.Total, s.Pending, s.Sample)
	}
	if len(s.Slots) != 1 || s.Slots[0].Count != 6 {
		t.Fatalf("khung = %+v", s.Slots)
	}
	if s.PerWeek == nil || *s.PerWeek != 1 {
		t.Fatalf("perWeek = %v", s.PerWeek)
	}
}

// Kênh đăng 3 lần mỗi ngày (20:45, 22:45, 00:45 VN) trong 10 ngày — mẫu thật
// của một đối thủ: 30 video chạm 2 tuần lịch nhưng chỉ trải ~9,2 ngày, nên nhịp
// là ~22 video/tuần chứ không phải 30/2 = 15. Không thứ nào có ≥ 3 video cùng
// giờ nên chưa có khung (đúng §12), mọi video là lẻ ngoài khung.
func TestPostingSlotsDailyChannelRate(t *testing.T) {
	var times []time.Time
	start := at(2026, 10, 20, 13, 45) // 20:45 VN thứ Ba 20/10
	for i := range 30 {
		times = append(times, start.Add(time.Duration(i/3)*24*time.Hour+time.Duration(i%3)*2*time.Hour))
	}
	s := PostingSlots("UCx", domain.ZoneVN, times, 0, loc(t, "vn"))
	if s.PerWeek == nil || *s.PerWeek < 21 || *s.PerWeek > 23 {
		t.Fatalf("perWeek = %v, muốn ~22 video/tuần", s.PerWeek)
	}
	if len(s.Slots) != 0 || s.OffSlot != 30 {
		t.Fatalf("slots = %+v offSlot = %d", s.Slots, s.OffSlot)
	}
}

// Mẫu ngắn hơn một tuần không bị ngoại suy: hai video cách nhau một giờ là
// một lần đăng tiếp trong tuần đó, không phải 168 video/tuần.
func TestPostingSlotsRateSpanFloor(t *testing.T) {
	s := PostingSlots("UCx", domain.ZoneVN, []time.Time{at(2026, 10, 26, 14, 0), at(2026, 10, 26, 15, 0)}, 0, loc(t, "vn"))
	if s.PerWeek == nil || *s.PerWeek != 1 {
		t.Fatalf("perWeek = %v, muốn 1", s.PerWeek)
	}
}
