// Package queue là hàng chờ chung của luồng clone (§3): mục `pick` lên đầu,
// mục `batch` (hàng loạt + tự lấy) đi theo vòng luân phiên kênh và chịu nhịp
// mỗi ngày reset lúc 00:00 giờ máy. File này chứa phần logic thuần không DB —
// port của queue.ts/queue-eta.test.ts (yt-factory 2c201e6) với luật nhịp mới:
// pick không tính nhịp (L4), mục của kênh đủ nhịp không giữ chỗ luồng.
package queue

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// Các giới hạn của workset (giữ nguyên bản TS).
const (
	OrderWorkset   = 500
	ItemsFetchCap  = 2000
	LaneCap        = 1000
	ETAHorizon     = 7 * 24 * time.Hour
	DefaultPace    = 5
	defaultSrcMin  = 5.0
	scenesFallback = 3.0
	redrawFactor   = 1.1
)

// Item là một mục hàng chờ ở dạng thuần (mirror CloneQueueItem, chỉ giữ
// trường lịch/ETA cần).
type Item struct {
	ID          string
	ChannelID   string
	Priority    string // "pick" | "batch"
	Position    float64
	DurationSec int64
	CreatedAt   time.Time
}

// Lane là hàng của một kênh trong nhóm batch.
type Lane struct {
	ChannelID     string
	Items         []Item // đã xếp theo position
	Pace          int
	UsedToday     int
	LastStart     time.Time // started_at mới nhất của kênh (0 = chưa chạy → đi trước)
	LastSequence  int64     // job.created_at mới nhất, chống hoà
	PausedChannel bool // Tự lấy tắt → chỉ khoá mục batch origin=auto
}

// ETA là giờ bắt đầu/kết thúc dự kiến của một mục; nil khi vượt chân trời
// 7 ngày hoặc mục bị chặn không giờ. Wait là nguyên nhân trì hoãn (nếu có).
type ETA struct {
	Start *time.Time
	Done  *time.Time
	Wait  string // "" | domain.WaitReason*
}

// ActiveJobETA là job đang giữ luồng, đưa vào mô phỏng ETA.
type ActiveJobETA struct {
	Minutes   float64 // độ dài nguồn phút (fallback 5)
	StartedAt time.Time
}

// Sample là một lần chạy bước thành công, nguyên liệu của ETA basis.
type Sample struct {
	Step          string
	Elapsed       time.Duration
	SourceSeconds float64
	FinishedAt    time.Time
	SceneCount    int // chỉ nghĩa với step "scenes"; ≤0 bỏ qua
}

// Basis là nhịp đo được: ms xử lý trên mỗi phút nguồn (bước theo độ dài) hoặc
// ms cố định (bước khác), và trung vị cảnh/phút cho ước lượng ảnh.
type Basis struct {
	Rates           map[string]time.Duration
	ScenesPerMinute float64
}

// Bước theo độ dài nguồn: rate lưu là thời gian xử lý mỗi PHÚT nguồn.
var sourceScaledSteps = map[string]bool{
	"download": true, "transcript": true, "images": true, "export": true,
}

// Fallback ms của từng bước (đúng số của bản TS); bước theo độ dài được nhân
// số phút nguồn.
var stepFallback = map[string]time.Duration{
	"download": 31_000, "backdrop": 60_000, "transcript": 48_000, "metadata": 5_000,
	"scenes": 10_000, "images": 100_000, "build": 20_000, "score": 15_000, "export": 37_000,
}

// etaSteps là danh sách bước theo thứ tự chuỗi (dùng chung với test).
var etaSteps = []string{
	"download", "backdrop", "transcript", "metadata", "scenes", "images",
	"build", "score", "export",
}

// DefaultBasis là basis khi chưa đo được lần chạy nào.
func DefaultBasis() Basis {
	rates := make(map[string]time.Duration, len(stepFallback))
	for step, ms := range stepFallback {
		rates[step] = ms * time.Millisecond
	}
	return Basis{Rates: rates, ScenesPerMinute: scenesFallback}
}

// ObservedBasis tính trung vị của tối đa 20 lần gần nhất mỗi bước (samples đã
// xếp mới → cũ), chuẩn hoá theo phút nguồn với bước theo độ dài, và trung vị
// cảnh/phút từ các sample "scenes" có SceneCount.
func ObservedBasis(samples []Sample) Basis {
	samples = append([]Sample(nil), samples...)
	sort.SliceStable(samples, func(i, j int) bool { return samples[i].FinishedAt.After(samples[j].FinishedAt) })
	basis := DefaultBasis()
	byStep := map[string][]float64{}
	var scenes []float64
	counts := map[string]int{}
	for _, s := range samples {
		if counts[s.Step] >= 20 {
			continue
		}
		counts[s.Step]++
		if s.Elapsed <= 0 {
			continue
		}
		minutes := s.SourceSeconds / 60
		if minutes <= 0 {
			minutes = defaultSrcMin
		}
		rate := float64(s.Elapsed)
		if sourceScaledSteps[s.Step] {
			rate /= minutes
		}
		byStep[s.Step] = append(byStep[s.Step], rate)
		if s.Step == "scenes" && s.SceneCount > 0 {
			scenes = append(scenes, float64(s.SceneCount)/minutes)
		}
	}
	for step, values := range byStep {
		if m, ok := median(values); ok {
			basis.Rates[step] = time.Duration(m)
		}
	}
	if m, ok := median(scenes); ok {
		basis.ScenesPerMinute = m
	}
	return basis
}

func median(values []float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2, true
	}
	return sorted[mid], true
}

// LocalMidnight trả 00:00 giờ máy của ngày chứa t.
func LocalMidnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// laneOrder sắp vòng luân phiên: kênh bắt đầu lâu nhất trước (chưa chạy đi
// đầu), hoà theo job mới nhất rồi channel_id.
func laneOrder(a, b Lane) int {
	if !a.LastStart.Equal(b.LastStart) {
		if a.LastStart.Before(b.LastStart) {
			return -1
		}
		return 1
	}
	if a.LastSequence != b.LastSequence {
		if a.LastSequence < b.LastSequence {
			return -1
		}
		return 1
	}
	if a.ChannelID < b.ChannelID {
		return -1
	}
	if a.ChannelID > b.ChannelID {
		return 1
	}
	return 0
}

// Order trả thứ tự drain toàn cục: mọi mục pick đứng trước (theo thứ tự
// enqueue, chống hoà bằng position), sau đó mỗi vòng một mục batch mỗi kênh
// theo laneOrder. Giới hạn workset mục.
func Order(lanes []Lane, workset int) []Item {
	var picks []Item
	for _, lane := range lanes {
		for _, item := range lane.Items {
			if item.Priority == "pick" {
				picks = append(picks, item)
			}
		}
	}
	sort.SliceStable(picks, func(i, j int) bool {
		if picks[i].Position != picks[j].Position {
			return picks[i].Position < picks[j].Position
		}
		if !picks[i].CreatedAt.Equal(picks[j].CreatedAt) {
			return picks[i].CreatedAt.Before(picks[j].CreatedAt)
		}
		return picks[i].ID < picks[j].ID
	})
	batchLanes := make([]Lane, 0, len(lanes))
	for _, lane := range lanes {
		var items []Item
		for _, item := range lane.Items {
			if item.Priority != "pick" {
				items = append(items, item)
			}
		}
		if len(items) > 0 {
			lane.Items = items
			batchLanes = append(batchLanes, lane)
		}
	}
	sort.SliceStable(batchLanes, func(i, j int) bool { return laneOrder(batchLanes[i], batchLanes[j]) < 0 })
	ordered := make([]Item, 0, workset)
	maxRank := 0
	for _, lane := range batchLanes {
		if len(lane.Items) > maxRank {
			maxRank = len(lane.Items)
		}
	}
outer:
	for rank := range maxRank {
		for _, lane := range batchLanes {
			if rank < len(lane.Items) {
				ordered = append(ordered, lane.Items[rank])
				if len(ordered) >= workset {
					break outer
				}
			}
		}
	}
	return append(picks[:min(len(picks), workset)], ordered[:min(len(ordered), max(0, workset-len(picks)))]...)
}

// EstimateInput đầu vào của mô phỏng ETA.
type EstimateInput struct {
	Now     time.Time
	Workers int
	Basis   Basis
	Active  []ActiveJobETA // job đang giữ luồng
	Paused  bool           // hàng chờ đang tạm dừng → mọi ETA nil + queue_paused
}

// Estimate mô phỏng lịch cho tối đa OrderWorkset mục trong Order: mỗi mục
// giữ một luồng suốt chuỗi (đoạn export tuần tự thêm trên một lane riêng),
// nhịp batch của kênh trôi qua 00:00 giờ máy khi hết, và mục pick không tiêu
// nhịp. Mục của kênh thiếu template/Tự lấy tắt không giữ chỗ luồng.
func Estimate(lanes []Lane, in EstimateInput) map[string]ETA {
	result := map[string]ETA{}
	if in.Now.IsZero() {
		in.Now = time.Now()
	}
	horizon := in.Now.Add(ETAHorizon).UnixMilli()
	if in.Basis.Rates == nil {
		in.Basis = DefaultBasis()
	}
	workers := in.Workers
	if workers < 1 {
		workers = 1
	}
	jobLanes := make([]int64, workers)
	for i := range jobLanes {
		jobLanes[i] = in.Now.UnixMilli()
	}
	for i, active := range in.Active {
		if i >= workers {
			break
		}
		var total time.Duration
		for _, step := range etaSteps {
			rate := in.Basis.Rates[step]
			if sourceScaledSteps[step] {
				total += time.Duration(float64(rate) * active.Minutes)
			} else {
				total += rate
			}
		}
		remaining := (total - in.Now.Sub(active.StartedAt)).Milliseconds()
		if remaining < 1000 {
			remaining = 1000
		}
		jobLanes[i] = in.Now.UnixMilli() + remaining
	}
	exportFreeAt := in.Now.UnixMilli()

	type budget struct {
		day  int64
		used int
		pace int
	}
	budgets := map[string]*budget{}
	for _, lane := range lanes {
		budgets[lane.ChannelID] = &budget{day: LocalMidnight(in.Now).UnixMilli(), used: lane.UsedToday, pace: lane.Pace}
	}
	laneByID := map[string]Lane{}
	for _, lane := range lanes {
		laneByID[lane.ChannelID] = lane
	}

	// Select the earliest runnable item before reserving either resource.
	// Future pace resets must not reserve today's job or export capacity.
	pending := Order(lanes, OrderWorkset)
	workerBlocked := false
	for len(pending) > 0 {
		worker := indexOfMin(jobLanes)
		best, startAt := -1, int64(0)
		for i, item := range pending {
			lane := laneByID[item.ChannelID]
			reason := ""
			switch {
			case in.Paused:
				reason = "queue_paused"
			case lane.PausedChannel && item.Priority != "pick":
				reason = "paused_channel"
			}
			if reason != "" {
				result[item.ID] = ETA{Wait: reason}
				continue
			}
			candidate := maxI64(jobLanes[worker], in.Now.UnixMilli())
			if item.Priority != "pick" {
				b := budgets[item.ChannelID]
				candidate = maxI64(candidate, b.day)
				day := LocalMidnight(time.UnixMilli(candidate).In(in.Now.Location())).UnixMilli()
				if day == b.day && b.used >= b.pace {
					candidate = LocalMidnight(time.UnixMilli(b.day).In(in.Now.Location())).AddDate(0, 0, 1).UnixMilli()
				}
			}
			if best < 0 || candidate < startAt {
				best, startAt = i, candidate
			}
		}
		if best < 0 {
			break
		}
		item := pending[best]
		paceWait := false
		if item.Priority != "pick" {
			b := budgets[item.ChannelID]
			paceWait = b.used >= b.pace || b.day > LocalMidnight(in.Now).UnixMilli()
			day := LocalMidnight(time.UnixMilli(startAt).In(in.Now.Location())).UnixMilli()
			if day > b.day {
				b.day, b.used = day, 0
			}
			b.used++
		}
		done := scheduleItem(item, startAt, in.Basis, &exportFreeAt)
		jobLanes[worker] = done
		eta := etaResult(startAt, done, horizon, &workerBlocked, !paceWait && startAt > in.Now.UnixMilli())
		if paceWait {
			eta.Wait = "pace_full"
		}
		result[item.ID] = eta
		pending = append(pending[:best], pending[best+1:]...)
	}
	return result
}

// etaResult gói kết quả: nil start khi vượt chân trời (kèm pace_full — mục
// chậm vì nhịp chứ không vì luồng); nil done khi done vượt chân trời.
// `workerBlocked` lật true sau mục workers_busy đầu tiên — các mục bị chặn
// luồng về sau ghi rotation (xếp sau lượt kênh của nó).
func etaResult(start, done, horizon int64, workerBlocked *bool, blockedByWorkers bool) ETA {
	if start > horizon {
		return ETA{Wait: "pace_full"}
	}
	s := time.UnixMilli(start)
	out := ETA{Start: &s}
	if done <= horizon {
		d := time.UnixMilli(done)
		out.Done = &d
	}
	if blockedByWorkers {
		if !*workerBlocked {
			*workerBlocked = true
			out.Wait = "workers_busy"
		} else {
			out.Wait = "rotation"
		}
	}
	return out
}

// scheduleItem trả thời điểm xong của một mục: các bước trước export chạy
// trên luồng job, export tuần tự trên lane chung, luồng job nhả lúc export
// xong (job giữ chỗ suốt chuỗi như bản TS).
func scheduleItem(item Item, start int64, basis Basis, exportFreeAt *int64) int64 {
	minutes := float64(item.DurationSec) / 60
	if minutes <= 0 {
		minutes = defaultSrcMin
	}
	var preExport time.Duration
	for _, step := range etaSteps {
		if step == "export" {
			continue
		}
		rate := basis.Rates[step]
		if sourceScaledSteps[step] {
			preExport += time.Duration(float64(rate) * minutes)
		} else {
			preExport += rate
		}
	}
	exportStart := maxI64(start+preExport.Milliseconds(), *exportFreeAt)
	done := exportStart + time.Duration(float64(basis.Rates["export"])*minutes).Milliseconds()
	*exportFreeAt = done
	return done
}

func indexOfMin(values []int64) int {
	idx := 0
	for i := 1; i < len(values); i++ {
		if values[i] < values[idx] {
			idx = i
		}
	}
	return idx
}

func minI64(values []int64) int64 {
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func maxI64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Move dời item id trong một lane (items đã xếp theo position): toTop lên
// đầu, beforeID rỗng xuống cuối, beforeID khác chèn ngay trước nó. Vị trí là
// midpoint phân số; khe < 1e-6 thì đánh số lại lane thành 1..n (bản TS làm
// trên DB; ở đây thuần trên slice). Lỗi khi id/beforeID không trong lane.
func Move(items []Item, id, beforeID string, toTop bool) ([]Item, error) {
	var item *Item
	lane := make([]Item, 0, len(items))
	for i := range items {
		if items[i].ID == id {
			item = &items[i]
			continue
		}
		lane = append(lane, items[i])
	}
	if item == nil {
		return nil, fmt.Errorf("mục hàng chờ không còn tồn tại")
	}
	var next float64
	switch {
	case toTop:
		if len(lane) == 0 {
			next = 1
		} else {
			next = lane[0].Position - 1
		}
	case beforeID == "":
		if len(lane) == 0 {
			next = 1
		} else {
			next = lane[len(lane)-1].Position + 1
		}
	default:
		idx := -1
		for i := range lane {
			if lane[i].ID == beforeID {
				idx = i
				break
			}
		}
		if idx < 0 {
			return nil, errors.New("mục đích không còn trong hàng chờ")
		}
		before := lane[idx]
		var prev *Item
		if idx > 0 {
			prev = &lane[idx-1]
		}
		if prev != nil && before.Position-prev.Position < 1e-6 {
			renumber(lane)
			for i := range lane {
				if lane[i].ID == beforeID {
					idx = i
					break
				}
			}
			before = lane[idx]
			if idx > 0 {
				prev = &lane[idx-1]
			} else {
				prev = nil
			}
		}
		if prev == nil {
			next = before.Position - 1
		} else {
			next = (prev.Position + before.Position) / 2
		}
	}
	item.Position = next
	out := make([]Item, 0, len(items))
	inserted := false
	for _, it := range lane {
		if !inserted && it.Position > item.Position {
			out = append(out, *item)
			inserted = true
		}
		out = append(out, it)
	}
	if !inserted {
		out = append(out, *item)
	}
	return out, nil
}

// renumber chuẩn hoá lane về vị trí nguyên 1..n.
func renumber(lane []Item) {
	for i := range lane {
		lane[i].Position = float64(i + 1)
	}
}

// InterleaveCandidates rotates the last competitor to the back, then takes
// one video per competitor per round. Input slices are never mutated.
func InterleaveCandidates(order []string, byRival map[string][]string, last string) []string {
	rotated := append([]string(nil), order...)
	for i, id := range rotated {
		if id == last {
			rotated = append(append([]string(nil), order[i+1:]...), order[:i+1]...)
			break
		}
	}
	out := []string{}
	for rank := 0; ; rank++ {
		added := false
		for _, id := range rotated {
			if rank < len(byRival[id]) {
				out = append(out, byRival[id][rank])
				added = true
			}
		}
		if !added {
			return out
		}
	}
}
