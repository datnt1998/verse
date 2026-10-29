package web

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
)

// packDetail là phần mở rộng của một gói trong GET /api/config/presets (D2):
// thẻ gói + ruột "Gói … gồm" + số đo + ghi đè văn phong + giọng đọc. Mọi con số
// đều đọc từ goi.json và các tệp luật của gói, không có số viết tay.
type packDetail struct {
	Desc      string            `json:"desc"`
	HasRules  bool              `json:"hasRules"`
	Available bool              `json:"available"`
	Layers    []packRuleLayer   `json:"layers"`
	Sections  []packRuleSection `json:"sections"`
	Metrics   []packMeasureRow  `json:"metrics"`
	Params    []packParam       `json:"params"`
	Overrides packOverrides     `json:"overrides"`
	Voice     string            `json:"voice"`
	Audio     packAudio         `json:"audio"`
}

// packAudio là khối `audio` của goi.json nguyên bản (D11 — audio theo gói):
// chế độ đọc và giọng gói khai, KHÔNG suy từ nhóm người nghe như `voice`.
// Gói không khai khoá nào thì trường đó rỗng.
type packAudio struct {
	Mode   string `json:"che_do"`
	Gender string `json:"giong"`
}

// packRuleLayer là một dòng của khối "Gói … gồm": lớp luật và số mục bắt sửa /
// cảnh báo của lớp đó.
type packRuleLayer struct {
	Key      string `json:"key"` // master | pack | measure
	Title    string `json:"title"`
	Desc     string `json:"desc"` // chữ cố định theo loại lớp (Câu 7)
	Errors   int    `json:"errors"`
	Warnings int    `json:"warnings"`
}

// packRuleSection là một § của lớp Master, kèm số mục bắt sửa / cảnh báo (như
// bảng "Luật chung · số mục bắt sửa / cảnh báo" của artboard) và mức nặng để tô màu.
type packRuleSection struct {
	Ref      string `json:"ref"`   // "Master §7"
	Title    string `json:"title"` // "Đầu chương và cuối chương"
	Level    string `json:"level"` // error | warning | ""
	Errors   int    `json:"errors"`
	Warnings int    `json:"warnings"`
}

// packMeasureRow là một chip "Số đo dùng để chấm" (cùng cách định dạng với /metrics).
type packMeasureRow struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Limit string `json:"limit"`
}

// packParam là một chip của hàng thông số trong chi tiết gói (Câu 7 bullet 4):
// chữ soạn ở máy chủ từ `do_do`, giao diện chỉ chọn icon theo `key`.
type packParam struct {
	Key  string `json:"key"` // chapters | dialogue | sentence | quiet | procedural | opening
	Text string `json:"text"`
}

// packOverrideRow là một mục voice gói ghi đè, chỉ giá trị mới (Câu 6).
type packOverrideRow struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// packOverrides là khối "Thay mặc định văn phong · N mục": các mục voice mà gói
// ghi đè (goi.json `voice` khác null).
type packOverrides struct {
	Count int               `json:"count"`
	Keys  []string          `json:"keys"`
	Rows  []packOverrideRow `json:"rows"`
}

var (
	ruleSectionRe = regexp.MustCompile(`^##\s+(\d+)\.\s*(.+)$`)
	ruleLabelRe   = regexp.MustCompile(`^\s*-\s`)
)

// buildPackDetail ghép toàn bộ phần chi tiết của một gói từ dữ liệu thật.
func buildPackDetail(p *assets.Pack) packDetail {
	out := packDetail{
		// `mo_ta` của goi.json là dòng mô tả dưới tên gói trên thẻ (Câu 7).
		Desc:      p.MoTa,
		Layers:    []packRuleLayer{},
		Sections:  []packRuleSection{},
		Metrics:   packMeasureRows(p),
		Params:    packParams(p),
		Overrides: packOverrides{Keys: []string{}, Rows: []packOverrideRow{}},
		Voice:     packVoiceLine(p),
		Audio:     packAudioRow(p),
	}
	var names map[string]string
	if ok, err := p.Section("nhan_luat", &names); err != nil || !ok {
		names = nil
	}
	layer := func(key, prefix, file string) {
		content := packRuleText(p, file)
		errors, warnings := ruleCounts(content)
		name := strings.TrimSpace(names[file])
		title := prefix
		if name != "" {
			title = prefix + " · " + name
		}
		desc := packLayerDesc(key, ruleSectionCount(content))
		out.Layers = append(out.Layers, packRuleLayer{Key: key, Title: title, Desc: desc, Errors: errors, Warnings: warnings})
		if key == "master" {
			if name == "" {
				name = "Master"
			}
			out.Sections = ruleSections(content, name)
		}
	}
	layer("master", "Luật chung", "master-rules-v3.md")
	layer("pack", "Luật thể loại", packRulesFile(p))
	layer("measure", "Cách đo", "dit-dang-van-phong.md")
	out.HasRules = out.Layers[0].Errors+out.Layers[0].Warnings > 0
	out.Available = out.HasRules && out.Layers[1].Errors+out.Layers[1].Warnings > 0 && len(out.Metrics) > 0
	out.Overrides = packVoiceOverrides(p)
	return out
}

// packLayerDesc là chữ cố định của từng lớp luật trong khối "Gói … gồm" (Câu 7):
// mô tả lớp không lấy từ gói, chỉ con số mục của Master đếm từ tệp luật.
func packLayerDesc(key string, sections int) string {
	switch key {
	case "master":
		return fmt.Sprintf("%d mục cho mọi truyện audio giọng AI tiếng Việt", sections)
	case "pack":
		return "Luật riêng của thể loại này. Không ghi đè mục nào của Master."
	case "measure":
		return "Cách đo thoại, câu, đoạn khi chấm tự động."
	}
	return ""
}

// ruleSectionCount đếm các mục "## N. …" có số hiệu từ 1 trở đi: §0 là "Cách
// dùng" nên không tính là một mục luật (Master v3 có 22 mục).
func ruleSectionCount(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if m := ruleSectionRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil && m[1] != "0" {
			count++
		}
	}
	return count
}

// packRulesFile là tệp luật riêng của gói (khác master và định dạng).
func packRulesFile(p *assets.Pack) string {
	for _, f := range p.Luat {
		if strings.HasPrefix(f.Name, "rules-") {
			return f.Name
		}
	}
	return ""
}

// packRuleText đọc nguyên văn một tệp luật đã nạp cùng gói; thiếu thì rỗng.
func packRuleText(p *assets.Pack, name string) string {
	for _, f := range p.Luat {
		if f.Name == name {
			return f.Content
		}
	}
	return ""
}

// ruleCounts đếm số mục có nhãn [LỖI] / [CẢNH BÁO] trong một tệp luật. Dòng
// định nghĩa nhãn ("- **[LỖI]:** …") là chú giải, không phải mục, nên bỏ.
func ruleCounts(text string) (errors, warnings int) {
	for _, line := range strings.Split(text, "\n") {
		if !ruleLabelRe.MatchString(line) {
			continue
		}
		if strings.Contains(line, "[LỖI]") && !strings.Contains(line, "**[LỖI]:**") {
			errors++
		}
		if strings.Contains(line, "[CẢNH BÁO]") && !strings.Contains(line, "**[CẢNH BÁO]:**") {
			warnings++
		}
	}
	return errors, warnings
}

// ruleSections tách các mục "## N. Tiêu đề" của lớp Master và chấm mức nặng theo
// nhãn bên trong mục. Tiêu đề không đánh số (ví dụ "## Nguồn …") bị bỏ.
func ruleSections(text, prefix string) []packRuleSection {
	out := []packRuleSection{}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		m := ruleSectionRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		body := strings.Join(lines[i+1:sectionEnd(lines, i+1)], "\n")
		errors, warnings := ruleCounts(body)
		level := ""
		switch {
		case errors > 0:
			level = "error"
		case warnings > 0:
			level = "warning"
		}
		out = append(out, packRuleSection{
			Ref:      fmt.Sprintf("%s §%s", prefix, m[1]),
			Title:    strings.TrimSpace(m[2]),
			Level:    level,
			Errors:   errors,
			Warnings: warnings,
		})
	}
	return out
}

// sectionEnd là chỉ số dòng đầu tiên sau mục bắt đầu ở from (heading kế tiếp).
func sectionEnd(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		if ruleSectionRe.MatchString(strings.TrimSpace(lines[i])) {
			return i
		}
	}
	return len(lines)
}

// packMeasureRows là chín dòng số đo của gói, cùng tên và chuẩn với /metrics.
// Dòng nào gói không khai ngưỡng thì bỏ; dòng "từ lặp" lấy ngưỡng chặt nhất.
func packMeasureRows(p *assets.Pack) []packMeasureRow {
	candidate, err := rules.PackDefaults(p)
	if err != nil {
		return []packMeasureRow{}
	}
	structured := rules.Effective(nil, candidate)
	if structured.Measurements == nil {
		return []packMeasureRow{}
	}
	config := *structured.Measurements
	rows := make([]packMeasureRow, 0, 9)
	add := func(key, name, limit string) { rows = append(rows, packMeasureRow{Key: key, Name: name, Limit: limit}) }
	if config.DialogueMax > 0 {
		add("thoai", "Tỷ lệ thoại", fmt.Sprintf("%s–%s%%", viNum(config.DialogueMin*100), viNum(config.DialogueMax*100)))
	}
	if config.SentenceMax > 0 {
		add("cau_dai_nhat", "Câu dài nhất", "≤ "+fmt.Sprint(config.SentenceMax))
	}
	if config.ParagraphMax > 0 {
		add("doan_dai_nhat", "Đoạn dài nhất", "≤ "+fmt.Sprint(config.ParagraphMax))
	}
	if config.QuietMax > 0 {
		add("quang_khong_thoai", "Quãng dài không thoại", "≤ "+fmt.Sprint(config.QuietMax))
	}
	if config.ProceduralMax > 0 {
		add("tu_thu_tuc", "Từ thủ tục", "≤ "+fmt.Sprint(config.ProceduralMax))
	}
	if len(config.EnglishWords) > 0 {
		add("tu_tieng_anh", "Từ tiếng Anh dễ đọc sai", "0")
	}
	if config.RepeatedMin > 0 {
		add("cum_lap", "Cụm 5 tiếng lặp lại", "0")
	}
	if config.OpeningWords > 0 && config.OpeningOverlapMax > 0 {
		add("trung_mo_dau", "Trùng mở đầu truyện cũ", "≤ "+viNum(config.OpeningOverlapMax*100)+"%")
	}
	if limit, ok := strictestFatigueLimit(structured.FatigueWords); ok {
		add("tu_lap", "Từ lặp", "≤ "+viNum(limit)+" / 1.000 tiếng")
	}
	return rows
}

// strictestFatigueLimit là ngưỡng thấp nhất trong bảng từ lặp (mục chặt nhất).
func strictestFatigueLimit(fatigue map[string]rules.FatiguePer1000) (float64, bool) {
	best, found := 0.0, false
	for word, limit := range fatigue {
		per1000 := limit.Per1000()
		if word == "" || per1000 <= 0 {
			continue
		}
		if !found || per1000 < best {
			best, found = per1000, true
		}
	}
	return best, found
}

// packVoiceOverrides là các mục voice gói ghi đè: khoá có giá trị khác null,
// sắp theo tên để kết quả ổn định. Mỗi dòng chỉ ghi giá trị mới (Câu 6).
func packVoiceOverrides(p *assets.Pack) packOverrides {
	keys := make([]string, 0, len(p.Voice))
	rows := make([]packOverrideRow, 0, len(p.Voice))
	for key, value := range p.Voice {
		if value == nil {
			continue
		}
		keys = append(keys, key)
		rows = append(rows, packOverrideRow{Key: key, Value: *value})
	}
	sort.Strings(keys)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
	return packOverrides{Count: len(keys), Keys: keys, Rows: rows}
}

// packParams là hàng thông số trong chi tiết gói (Câu 7 bullet 4): chỉ đọc
// `do_do`/`thoai`, đúng thứ tự artboard, bỏ dòng nào gói không khai ngưỡng.
func packParams(p *assets.Pack) []packParam {
	out := make([]packParam, 0, 6)
	add := func(key, text string) { out = append(out, packParam{Key: key, Text: text}) }
	if p.SoChuong > 0 {
		add("chapters", fmt.Sprintf("%d chương", p.SoChuong))
	}
	if p.Thoai != nil && p.Thoai.Max > 0 {
		add("dialogue", fmt.Sprintf("Thoại %s–%s%%", viNum(float64(p.Thoai.Min)), viNum(float64(p.Thoai.Max))))
	}
	candidate, err := rules.PackDefaults(p)
	if err != nil {
		return out
	}
	structured := rules.Effective(nil, candidate)
	if structured.Measurements == nil {
		return out
	}
	config := *structured.Measurements
	if config.SentenceMax > 0 && config.ParagraphMax > 0 {
		add("sentence", fmt.Sprintf("Câu ≤ %d tiếng · đoạn ≤ %d tiếng", config.SentenceMax, config.ParagraphMax))
	}
	if config.QuietMax > 0 {
		add("quiet", fmt.Sprintf("Quãng không thoại ≤ %d tiếng", config.QuietMax))
	}
	if config.ProceduralMax > 0 {
		add("procedural", fmt.Sprintf("Từ thủ tục ≤ %d mỗi chương", config.ProceduralMax))
	}
	if config.OpeningWords > 0 && config.OpeningOverlapMax > 0 {
		add("opening", fmt.Sprintf("%d tiếng mở đầu trùng truyện cũ ≤ %s%%", config.OpeningWords, viNum(config.OpeningOverlapMax*100)))
	}
	return out
}

// packShort là tên gói trên thẻ (Câu 12): `ten_ngan`; gói cũ thiếu thì bỏ tiền
// tố "Gói " và phần sau dấu phẩy của `ten`.
func packShort(p *assets.Pack) string {
	if short := strings.TrimSpace(p.TenNgan); short != "" {
		return short
	}
	name := strings.TrimSpace(p.Ten)
	if comma := strings.Index(name, ","); comma >= 0 {
		name = name[:comma]
	}
	return strings.TrimSpace(strings.TrimPrefix(name, "Gói "))
}

// packMeta là dòng phụ của thẻ gói: số chương + khoảng thoại, bỏ khoảng
// tiếng/chương (Câu 7).
func packMeta(p *assets.Pack) string {
	if p.Thoai == nil || p.Thoai.Max <= 0 {
		return fmt.Sprintf("%d chương", p.SoChuong)
	}
	return fmt.Sprintf("%d chương · thoại %s–%s%%", p.SoChuong, viNum(float64(p.Thoai.Min)), viNum(float64(p.Thoai.Max)))
}

// packAudioRow đọc thẳng khối `audio` của goi.json (D11). Khác packVoiceLine:
// không điền `giong` từ nhóm người nghe — giao diện cần biết gói KHAI gì, còn
// mặc định suy ra vẫn do engine áp lúc phân vai. goi.json hỏng/không khai audio
// thì trả zero (che_do "", giong "").
func packAudioRow(p *assets.Pack) packAudio {
	var row packAudio
	if _, err := p.Section("audio", &row); err != nil {
		return packAudio{}
	}
	return row
}

// packVoiceLine là câu "Giọng đọc theo gói: …", lấy từ khối audio của gói (P1-4)
// với cùng mặc định mà phân vai dùng (giọng suy từ nhóm người nghe).
func packVoiceLine(p *assets.Pack) string {
	defaults, err := audio.ReadPackDefaults(p, p.NguoiNghe)
	if err != nil || defaults == nil {
		return ""
	}
	if defaults.Mode == audio.ModeCast {
		return "Nhiều giọng, chia vai theo nhân vật"
	}
	switch defaults.Gender {
	case "nu":
		return "Một giọng nữ kể cả truyện"
	case "nam":
		return "Một giọng nam kể cả truyện"
	default:
		return "Một giọng kể cả truyện"
	}
}
