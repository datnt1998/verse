package web

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
)

// metricRow là một dòng "Đo bằng máy" (D6): tên · số · đơn vị · chuẩn · đạt/lệch.
// value là số trần (null khi phép đo không áp cho chương đó — mở đầu trùng chỉ đo
// chương 1), unit và limit là chữ để UI hiện nguyên văn ("tiếng", "≤ 40").
// severity/ref chỉ có khi tệp luật của gói gán mức nặng cho phép đo.
type metricRow struct {
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	Value    *float64 `json:"value"`
	Unit     string   `json:"unit"`
	Limit    string   `json:"limit"`
	OK       bool     `json:"ok"`
	Detail   string   `json:"detail,omitempty"`
	Severity string   `json:"severity,omitempty"`
	Ref      string   `json:"ref,omitempty"`
}

// tiredWord là dòng "Từ mệt nhiều nhất · “x”": từ mệt có mật độ cao nhất trong
// chương (cùng từ mà bảng luật đã chọn) kèm ngưỡng của từ đó.
type tiredWord struct {
	Word    string  `json:"word"`
	Per1000 float64 `json:"per1000"`
	Limit   float64 `json:"limit"`
	OK      bool    `json:"ok"`
}

type chapterMetrics struct {
	N         int         `json:"n"`
	Metrics   []metricRow `json:"metrics"`
	TiredWord *tiredWord  `json:"tiredWord"`
}

const (
	metricsLastN = 3
	metricsAll   = "all"
)

// serveStoryMetrics xử lý GET /metrics?chapters=last3|all (D6): chín số đo của
// gói thể loại cho từng chương đã viết, cùng cách đo với lúc kiểm chương.
// 授权已在 serveStories 的 storyAccessAllowed 完成（GET 需可读）。
func (g *Gateway) serveStoryMetrics(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "không hỗ trợ phương thức này")
		return
	}
	scope := strings.TrimSpace(r.URL.Query().Get("chapters"))
	if scope == "" {
		scope = "last3"
	}
	if scope != "last3" && scope != metricsAll {
		jsonError(w, http.StatusBadRequest, "chapters phải là last3 hoặc all")
		return
	}
	pack, err := storyPack(st, sr)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được gói thể loại: "+err.Error())
		return
	}
	candidate, err := rules.PackDefaults(pack)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được phép đo của gói: "+err.Error())
		return
	}
	snap, _ := sr.Store().UserRules.Load()
	structured := rules.Effective(snap, candidate)
	if structured.Measurements == nil {
		// Gói không khai do_do: không có gì để đo, trả danh sách rỗng thay vì số bịa.
		writeJSON(w, http.StatusOK, map[string]any{"chapters": []chapterMetrics{}})
		return
	}
	config := *structured.Measurements
	chapters, err := sr.Chapters()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được danh sách chương: "+err.Error())
		return
	}
	written := make([]int, 0, len(chapters))
	for _, c := range chapters {
		if c.Completed {
			written = append(written, c.Chapter)
		}
	}
	if scope == "last3" && len(written) > metricsLastN {
		written = written[len(written)-metricsLastN:]
	}
	out := make([]chapterMetrics, 0, len(written))
	refs := packMeasureRefs(pack)
	for _, n := range written {
		text, err := sr.ChapterText(n, "committed")
		if err != nil || strings.TrimSpace(text.Text) == "" {
			continue
		}
		stats := rules.MeasureChapterStats(text.Text, config, rules.SiblingOpeningsFor(sr.EngineDir(), n))
		out = append(out, chapterMetrics{
			N:         n,
			Metrics:   buildChapterMetrics(n, stats, config, refs),
			TiredWord: tiredWordOf(text.Text, structured.FatigueWords),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"chapters": out})
}

// buildChapterMetrics dựng chín dòng số đo đúng thứ tự, tên, đơn vị và chuẩn
// của bảng D6 (22-brief line 575). Dòng nào gói không khai ngưỡng thì bỏ. Dòng
// "từ mệt nhiều nhất" KHÔNG nằm trong chín dòng này — nó là tiredWord.
func buildChapterMetrics(chapter int, stats rules.ChapterStats, config rules.Measurements, refs map[string]metricRef) []metricRow {
	rows := make([]metricRow, 0, 9)
	add := func(key, name string, value *float64, unit, limit string, ok bool, detail string) {
		row := metricRow{Key: key, Name: name, Value: value, Unit: unit, Limit: limit, OK: ok, Detail: detail}
		if ref, found := refs[key]; found {
			row.Severity, row.Ref = ref.Severity, ref.Ref
		}
		rows = append(rows, row)
	}
	count := func(n int) *float64 { v := float64(n); return &v }
	over := func(n, longest int, unit, key, name string) {
		detail := ""
		if n > 0 {
			detail = fmt.Sprintf("dài nhất %d tiếng", longest)
		}
		add(key, name, count(n), unit, "0", n == 0, detail)
	}
	if config.DialogueMax > 0 {
		ratio := math.Round(stats.DialogueRatio * 100)
		add("thoai", "Tỷ lệ thoại", &ratio, "%",
			fmt.Sprintf("%s–%s%%", viNum(config.DialogueMin*100), viNum(config.DialogueMax*100)),
			stats.DialogueRatio >= config.DialogueMin && stats.DialogueRatio <= config.DialogueMax, "")
	}
	if config.SentenceMax > 0 {
		over(stats.SentencesOver, stats.MaxSentence, "câu", "cau_dai_nhat", "Câu quá "+strconv.Itoa(config.SentenceMax)+" tiếng")
	}
	if config.ParagraphMax > 0 {
		over(stats.ParagraphsOver, stats.MaxParagraph, "đoạn", "doan_dai_nhat", "Đoạn quá "+strconv.Itoa(config.ParagraphMax)+" tiếng")
	}
	if config.QuietMax > 0 {
		over(stats.QuietOver, stats.QuietWords, "quãng", "quang_khong_thoai", "Quãng không thoại quá "+strconv.Itoa(config.QuietMax)+" tiếng")
	}
	if config.ProceduralMax > 0 {
		add("tu_thu_tuc", "Từ thủ tục", count(stats.Procedural), "từ", "≤ "+strconv.Itoa(config.ProceduralMax),
			stats.Procedural <= config.ProceduralMax, "")
	}
	if len(config.EnglishWords) > 0 {
		add("tu_tieng_anh", "Từ tiếng Anh dễ đọc sai", count(stats.EnglishCount), "từ", "0", stats.EnglishCount == 0, "")
	}
	if config.DayOrdinal {
		add("ngay_thu", "“ngày thứ” + số", count(stats.DayLines), "dòng", "0", stats.DayLines == 0, "")
	}
	if config.RepeatedMin > 0 {
		add("cum_lap", "Cụm 5 tiếng lặp lại", count(stats.RepeatedGrams), "cụm", "0", stats.RepeatedGrams == 0, "")
	}
	if config.OpeningWords > 0 && config.OpeningOverlapMax > 0 {
		limit := "≤ " + viNum(config.OpeningOverlapMax*100) + "% · chỉ đo chương 1"
		if chapter != 1 {
			// Mở đầu chỉ so được ở chương 1: chương khác không có số, giao diện hiện "—".
			add("trung_mo_dau", "Mở đầu trùng truyện cũ", nil, "%", limit, true, "")
		} else {
			worst := 0.0
			for _, opening := range stats.Openings {
				worst = max(worst, opening.Ratio)
			}
			percent := math.Round(worst * 100)
			add("trung_mo_dau", "Mở đầu trùng truyện cũ", &percent, "%", limit, worst <= config.OpeningOverlapMax, "")
		}
	}
	return rows
}

// tiredWordOf là dòng "Từ mệt nhiều nhất · “x”": chọn từ có mật độ cao nhất
// trong chương; chương không dùng từ nào trong bảng thì lấy từ có ngưỡng chặt
// nhất để dòng vẫn ổn định (số 0, đạt). Ngưỡng là lần/1.000 tiếng như
// appendFatigueWords. Gói không khai bảng từ mệt → nil.
func tiredWordOf(text string, fatigue map[string]rules.FatiguePer1000) *tiredWord {
	if len(fatigue) == 0 {
		return nil
	}
	densities := rules.FatigueDensities(text, fatigue)
	if len(densities) > 0 {
		worst := densities[0]
		return &tiredWord{
			Word:    worst.Word,
			Per1000: round2(worst.Per1000),
			Limit:   worst.Limit,
			OK:      worst.Per1000 <= worst.Limit,
		}
	}
	words := make([]string, 0, len(fatigue))
	for word := range fatigue {
		if word != "" && fatigue[word].Per1000() > 0 {
			words = append(words, word)
		}
	}
	if len(words) == 0 {
		return nil
	}
	sort.Slice(words, func(i, j int) bool {
		a, b := fatigue[words[i]].Per1000(), fatigue[words[j]].Per1000()
		if a != b {
			return a < b
		}
		return words[i] < words[j]
	})
	return &tiredWord{Word: words[0], Per1000: 0, Limit: fatigue[words[0]].Per1000(), OK: true}
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// viNum in số theo cách đọc Việt Nam: thập phân bằng dấu phẩy, bỏ số 0 thừa.
func viNum(v float64) string {
	text := strconv.FormatFloat(round2(v), 'f', -1, 64)
	return strings.ReplaceAll(text, ".", ",")
}
