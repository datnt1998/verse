package rivals

import (
	"fmt"
	"sort"
	"time"

	_ "time/tzdata" // binary chạy được kể cả khi hệ thống thiếu tzdata

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Thuật toán khung đăng lặp lại (§12): thuần hàm trên giờ đăng chính xác của
// mẫu video mới nhất — không I/O, không đồng hồ (số tuần suy ra từ chính mẫu).
//
//  1. Đổi mọi giờ đăng sang múi giờ đang xem; thứ 0 = T2 … 6 = CN; tuần bắt
//     đầu thứ Hai theo múi giờ đó.
//  2. weeksTotal = số tuần từ lần đăng cũ nhất tới mới nhất (tính cả hai đầu).
//  3. Mỗi thứ: gom theo giờ, gộp các giờ liền nhau thành khung [start, end).
//  4. Khung lặp lại khi ≥ 3 video trên ≥ 2 tuần khác nhau; còn lại vào offSlot.
//  5. Độ đều: weekly (đủ mọi tuần) → most_weeks (≥ 75%) → new (lần đầu trong
//     3 tuần gần nhất) → biweekly.
//  6. Gộp các thứ cùng [start,end) và cùng độ đều thành một khung.
//  7. Sắp theo count giảm, thứ đầu tiên, giờ bắt đầu; khung đầu là isTop.
//  8. grid[7][12] = số video theo thứ × khối 2 giờ; perWeek = số khoảng giữa
//     các lần đăng (total − 1) chia độ dài thật của mẫu tính theo tuần, tối
//     thiểu 1 tuần để mẫu ngắn không bị ngoại suy (null khi < 2 video).

// ZoneLocation trả *time.Location của một múi giờ xem khung đăng.
func ZoneLocation(zone string) (*time.Location, error) {
	switch zone {
	case domain.ZoneVN:
		return time.LoadLocation("Asia/Ho_Chi_Minh")
	case domain.ZoneNY:
		return time.LoadLocation("America/New_York")
	default:
		return nil, fmt.Errorf("múi giờ không hỗ trợ: %s", zone)
	}
}

// zonedParts đổi một thời điểm sang loc rồi trả thứ (0 = T2 … 6 = CN), giờ và
// chỉ số tuần (bắt đầu thứ Hai, tính theo lịch của loc). daysFromCivil tính
// trên số ngày lịch nên DST chỉ đổi thứ/giờ chứ không làm trôi tuần.
func zonedParts(t time.Time, loc *time.Location) (weekday, hour, week int) {
	lt := t.In(loc)
	y, m, d := lt.Date()
	z := daysFromCivil(y, int(m), d)
	weekday = mod(z+3, 7) // 1970-01-01 (z=0) là thứ Năm → 3
	return weekday, lt.Hour(), floorDiv(z-weekday, 7)
}

// daysFromCivil trả số ngày từ 1970-01-01 của một ngày lịch Gregory.
func daysFromCivil(y, m, d int) int {
	if m <= 2 {
		y--
	}
	era := floorDiv(y, 400)
	yoe := y - era*400
	mp := m - 3
	if m <= 2 {
		mp = m + 9
	}
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func mod(a, b int) int {
	m := a % b
	if m < 0 {
		m += b
	}
	return m
}

// win là một khung giờ liền nhau trong một thứ: [start, end) theo giờ.
type win struct {
	weekday  int
	start    int // giờ mở khung
	end      int // giờ đóng khung (không gồm)
	count    int
	weeks    map[int]bool // các tuần có ít nhất một video trong khung
	firstHit int          // tuần sớm nhất có video
}

// PostingSlots chạy thuật toán §12 trên giờ đăng chính xác của mẫu video mới
// nhất. times là các lần đăng đã đọc được (sample của domain.PostingSample);
// pending là số video của mẫu chưa đọc giờ.
func PostingSlots(channelID string, zone domain.PostingZone, times []time.Time, pending int, loc *time.Location) *domain.PostingSchedule {
	sched := &domain.PostingSchedule{
		ChannelID: channelID,
		Zone:      zone,
		Sample:    domain.PostingSample,
		Total:     len(times),
		Pending:   pending,
	}
	if len(times) == 0 {
		return sched
	}
	type point struct {
		weekday, hour, week int
	}
	pts := make([]point, 0, len(times))
	weekMin, weekMax := 0, 0
	oldest, newest := times[0], times[0]
	for i, t := range times {
		wd, h, w := zonedParts(t, loc)
		pts = append(pts, point{wd, h, w})
		sched.Grid[wd][h/2]++
		if i == 0 || w < weekMin {
			weekMin = w
		}
		if i == 0 || w > weekMax {
			weekMax = w
		}
		if t.Before(oldest) {
			oldest = t
		}
		if t.After(newest) {
			newest = t
		}
	}
	sched.WeeksTotal = weekMax - weekMin + 1
	if sched.Total >= 2 {
		// Số tuần lịch chạm tới (WeeksTotal) không phải độ dài mẫu: 30 video
		// trong 9 ngày chạm 2 tuần lịch nhưng nhịp thật là ~22 video/tuần.
		span := max(newest.Sub(oldest), 7*24*time.Hour)
		perWeek := float64(sched.Total-1) * float64(7*24*time.Hour) / float64(span)
		sched.PerWeek = &perWeek
	}

	// Gom giờ theo thứ → gộp các giờ liền nhau thành khung.
	byWeekdayHour := [7]map[int]*win{}
	for i := range byWeekdayHour {
		byWeekdayHour[i] = map[int]*win{}
	}
	for _, p := range pts {
		w := byWeekdayHour[p.weekday][p.hour]
		if w == nil {
			w = &win{weekday: p.weekday, start: p.hour, end: p.hour + 1, weeks: map[int]bool{}, firstHit: p.week}
			byWeekdayHour[p.weekday][p.hour] = w
		}
		w.count++
		w.weeks[p.week] = true
		if p.week < w.firstHit {
			w.firstHit = p.week
		}
	}
	var windows []*win
	for wd := range 7 {
		hours := make([]int, 0, len(byWeekdayHour[wd]))
		for h := range byWeekdayHour[wd] {
			hours = append(hours, h)
		}
		sort.Ints(hours)
		var cur *win
		for _, h := range hours {
			part := byWeekdayHour[wd][h]
			if cur != nil && part.start == cur.end {
				// Giờ liền nhau cùng khung: gộp đếm và tuần.
				cur.end = part.end
				cur.count += part.count
				for wk := range part.weeks {
					cur.weeks[wk] = true
				}
				if part.firstHit < cur.firstHit {
					cur.firstHit = part.firstHit
				}
				continue
			}
			cur = &win{weekday: wd, start: part.start, end: part.end, count: part.count,
				weeks: part.weeks, firstHit: part.firstHit}
			windows = append(windows, cur)
		}
	}

	// Khung lặp lại: ≥ 3 video trên ≥ 2 tuần. Độ đều theo thứ tự weekly →
	// most_weeks → new → biweekly.
	type keyed struct {
		start, end int
		rhythm     domain.PostingRhythm
	}
	groups := map[keyed]*domain.PostingSlot{}
	hitSets := map[keyed]map[int]bool{}
	firstHits := map[keyed]int{}
	for _, w := range windows {
		if w.count < 3 || len(w.weeks) < 2 {
			sched.OffSlot += w.count
			continue
		}
		weeksHit := len(w.weeks)
		sinceWeeks := weekMax - w.firstHit + 1
		var rhythm domain.PostingRhythm
		switch {
		case weeksHit == sched.WeeksTotal:
			rhythm = domain.RhythmWeekly
		case float64(weeksHit)/float64(sched.WeeksTotal) >= 0.75:
			rhythm = domain.RhythmMostWeeks
		case sinceWeeks <= 3:
			rhythm = domain.RhythmNew
		default:
			rhythm = domain.RhythmBiweekly
		}
		k := keyed{w.start, w.end, rhythm}
		slot := groups[k]
		if slot == nil {
			slot = &domain.PostingSlot{
				Start:      fmt.Sprintf("%02d:00", w.start),
				End:        fmt.Sprintf("%02d:00", w.end%24),
				WeeksTotal: sched.WeeksTotal,
				Rhythm:     rhythm,
			}
			groups[k] = slot
			hitSets[k] = map[int]bool{}
			firstHits[k] = w.firstHit
		}
		slot.Weekdays = append(slot.Weekdays, w.weekday)
		slot.Count += w.count
		for wk := range w.weeks {
			hitSets[k][wk] = true
		}
		if w.firstHit < firstHits[k] {
			firstHits[k] = w.firstHit
		}
	}
	for k, slot := range groups {
		sort.Ints(slot.Weekdays)
		slot.WeeksHit = len(hitSets[k])
		slot.SinceWeeks = weekMax - firstHits[k] + 1
		sched.Slots = append(sched.Slots, *slot)
	}
	sort.Slice(sched.Slots, func(i, j int) bool {
		a, b := sched.Slots[i], sched.Slots[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Weekdays[0] != b.Weekdays[0] {
			return a.Weekdays[0] < b.Weekdays[0]
		}
		return a.Start < b.Start
	})
	if len(sched.Slots) > 0 {
		sched.Slots[0].IsTop = true
	}
	return sched
}
