package performance

import (
	"sort"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// RotationWeights tính số lượt mỗi template trong một vòng luân phiên (1 hoặc 2,
// không bao giờ hơn gấp đôi). manual là "Cho chạy nhiều hơn" do bạn bấm; khi kênh
// ưu tiên Lượt xem và đã có đủ MinVideos video có số 7 ngày, template có trung vị
// lượt xem 7 ngày cao hơn trung vị cả kênh từ MinRatio lần (và ≥ MinGroup video)
// cũng được gấp đôi. Chưa đủ số thì chạy như Đều đặn.
func RotationWeights(manual map[string]int, priority string, views7d map[string][]int64) map[string]int {
	out := map[string]int{}
	for id, w := range manual {
		if w >= 2 {
			out[id] = 2
		}
	}
	if priority != domain.PriorityViews {
		return out
	}
	all := []int64{}
	for _, v := range views7d {
		all = append(all, v...)
	}
	if len(all) < MinVideos {
		return out
	}
	overall := Median(all)
	if overall <= 0 {
		return out
	}
	for id, v := range views7d {
		if len(v) >= MinGroup && float64(Median(v)) >= MinRatio*float64(overall) {
			out[id] = 2
		}
	}
	return out
}

// RotationSequence là danh sách luân phiên: mỗi template một lượt theo thứ tự,
// rồi vòng thứ hai chỉ gồm template có trọng số 2. Con trỏ luân phiên chạy trên
// danh sách này.
func RotationSequence(templates []domain.ChannelTemplate, weights map[string]int) []domain.ChannelTemplate {
	out := append([]domain.ChannelTemplate(nil), templates...)
	for _, t := range templates {
		if weights[t.ID] >= 2 {
			out = append(out, t)
		}
	}
	return out
}

// OrderByScore xếp id theo điểm nổi trội giảm dần (ổn định; id không có điểm đứng cuối).
func OrderByScore(ids []string, scores map[string]float64) []string {
	out := append([]string(nil), ids...)
	sort.SliceStable(out, func(i, j int) bool {
		a, aok := scores[out[i]]
		b, bok := scores[out[j]]
		if aok != bok {
			return aok
		}
		return a > b
	})
	return out
}
