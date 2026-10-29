package audio

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// 越南语文本的确定性规范化：数字、序数、小数、百分比、金额、日期、时间、单位、
// 符号、缩写，加上项目级读音词典（.ainovel/cach-doc.json）。
//
// 模型只负责"把章节拆成朗读段、判断谁在说话"，凡是"同一个字必须每次读成同一串
// 音"的活都在这里用 Go 做完：结果可测、可复现，换模型也不会漂。

// NormalizeOptions 是"tự chuẩn hoá"的四个开关（设置 › Cách đọc · cả dự án）。
// 缺省全开；关闭某一项时该类文本原样保留，由模型自行处理。
type NormalizeOptions struct {
	Numbers       bool `json:"numbers"`       // số thành chữ
	Dates         bool `json:"dates"`         // ngày / giờ
	Money         bool `json:"money"`         // tiền, phần trăm
	Symbols       bool `json:"symbols"`       // ký hiệu (&, +, ×, khoảng "-")
	Abbreviations bool `json:"abbreviations"` // viết tắt (TP.HCM, NXB…)
	ChapterTitles bool `json:"chapterTitles"` // đọc tên chương ở đầu
	DropBrackets  bool `json:"dropBrackets"`  // bỏ ghi chú trong ngoặc vuông
}

// DefaultNormalizeOptions 是新建项目的缺省值：全开（对应 10-audio §2 的默认开关）。
func DefaultNormalizeOptions() NormalizeOptions {
	return NormalizeOptions{
		Numbers: true, Dates: true, Money: true, Symbols: true,
		Abbreviations: true, ChapterTitles: true, DropBrackets: true,
	}
}

// Pronunciation 是项目读音词典的一条（API 形状 [{written, spoken}]）。
type Pronunciation struct {
	Written string `json:"written"`
	Spoken  string `json:"spoken"`
}

// Change 记录一处规范化，供界面下划线 + 悬停显示原文（"90 giây → chín mươi giây"）。
type Change struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"` // number|decimal|ordinal|percent|money|date|time|unit|symbol|abbreviation|pronunciation|bracket
}

// Normalize 规范化一段文本，返回新文本与逐处变化。
// 变化按出现顺序排列；词典条目优先于缩写与数字（"MC" 被词典改成 "em-xi" 后不再当缩写）。
func Normalize(text string, opts NormalizeOptions, dict []Pronunciation) (string, []Change) {
	n := &normalizer{text: text, opts: opts}
	if opts.DropBrackets {
		n.apply(bracketRe, "bracket", func(sub []string) (string, string, string) {
			return "", sub[0], ""
		})
	}
	n.applyDictionary(dict)
	if opts.Dates {
		n.normalizeDates()
		n.normalizeTimes()
	}
	// 斜杠必须在数字读法之前处理：日期已先转成"ngày … tháng …"，剩下的 "/"
	// 还要按语境决定读法（"A/B" → "A hoặc B"；真分数 → "trên"；其余用停顿）。
	if opts.Symbols {
		n.normalizeSlashes()
	}
	if opts.Money {
		n.normalizeMoney()
		n.normalizePercent()
	}
	if opts.Abbreviations {
		n.normalizeAbbreviations()
	}
	// 顺序有讲究：区间（3-5）先吃掉连字符，单位（5 km）再吃掉数字+单位，
	// 剩下裸数字才走通用读法，最后补"单位没有数字"的残局。
	if opts.Numbers {
		n.normalizeRanges()
	}
	if opts.Symbols {
		n.normalizeUnits()
	}
	if opts.Numbers {
		n.normalizeNumbers()
	}
	if opts.Symbols {
		n.normalizeLooseUnits()
		n.normalizeSymbols()
	}
	return strings.Join(strings.Fields(n.text), " "), n.changes
}

type normalizer struct {
	text    string
	opts    NormalizeOptions
	changes []Change
}

// apply 用正则逐处替换；fn 返回 (替换文本, 记录用原文, 记录用新文)。
// 替换文本与原文相同的匹配会被跳过，因此不会污染 changes。
func (n *normalizer) apply(re *regexp.Regexp, kind string, fn func(sub []string) (string, string, string)) {
	matches := re.FindAllStringSubmatchIndex(n.text, -1)
	if len(matches) == 0 {
		return
	}
	var b strings.Builder
	b.Grow(len(n.text))
	last := 0
	for _, m := range matches {
		whole := n.text[m[0]:m[1]]
		sub := make([]string, 0, len(m)/2)
		for i := 0; i < len(m); i += 2 {
			if m[i] < 0 {
				sub = append(sub, "")
				continue
			}
			sub = append(sub, n.text[m[i]:m[i+1]])
		}
		rep, from, to := fn(sub)
		if rep == whole {
			continue
		}
		b.WriteString(n.text[last:m[0]])
		b.WriteString(rep)
		last = m[1]
		if from != "" || to != "" {
			n.changes = append(n.changes, Change{From: from, To: to, Kind: kind})
		}
	}
	if last == 0 {
		return
	}
	b.WriteString(n.text[last:])
	n.text = b.String()
}

func (n *normalizer) applyDictionary(dict []Pronunciation) {
	for _, e := range dict {
		w := strings.TrimSpace(e.Written)
		s := strings.TrimSpace(e.Spoken)
		if w == "" || s == "" || strings.EqualFold(w, s) {
			continue
		}
		re := regexp.MustCompile(`(?i)(^|[^\p{L}\p{N}_])(` + regexp.QuoteMeta(w) + `)([^\p{L}\p{N}_]|$)`)
		n.apply(re, "pronunciation", func(sub []string) (string, string, string) {
			return sub[1] + s + sub[3], sub[2], s
		})
	}
}

var bracketRe = regexp.MustCompile(`\[[^\[\]]*\]`)

var (
	dateFullRe = regexp.MustCompile(`\b(\d{1,2})[/\-.](\d{1,2})[/\-.](\d{4})\b`)
	dateShort  = regexp.MustCompile(`\b(\d{1,2})/(\d{1,2})\b`)
	yearAfter  = regexp.MustCompile(`(?i)\b(?:năm|year)\s+(\d{3,4})\b`)
	timeColon  = regexp.MustCompile(`\b(\d{1,2}):(\d{2})(?::(\d{2}))?\b`)
	timeH      = regexp.MustCompile(`\b(\d{1,2})h(\d{2})?\b`)
	timeWord   = regexp.MustCompile(`\b(\d{1,2})\s*(?:giờ|h)\s*(\d{1,2})\b`)
	ordinalRe  = regexp.MustCompile(`(?i)\bthứ\s+(\d{1,3})\b`)
)

func (n *normalizer) normalizeDates() {
	n.apply(dateFullRe, "date", func(sub []string) (string, string, string) {
		d, _ := strconv.Atoi(sub[1])
		m, _ := strconv.Atoi(sub[2])
		y, _ := strconv.Atoi(sub[3])
		out := "ngày " + viNumberWords(uint64(d)) + " tháng " + viNumberWords(uint64(m)) +
			" năm " + viNumberWords(uint64(y))
		return out, sub[0], out
	})
	n.apply(dateShort, "date", func(sub []string) (string, string, string) {
		d, _ := strconv.Atoi(sub[1])
		m, _ := strconv.Atoi(sub[2])
		if d < 1 || d > 31 || m < 1 || m > 12 {
			return sub[0], "", ""
		}
		out := "ngày " + viNumberWords(uint64(d)) + " tháng " + viNumberWords(uint64(m))
		return out, sub[0], out
	})
	n.apply(yearAfter, "date", func(sub []string) (string, string, string) {
		y, err := strconv.ParseUint(sub[1], 10, 64)
		if err != nil {
			return sub[0], "", ""
		}
		return "năm " + viNumberWords(y), sub[0], "năm " + viNumberWords(y)
	})
}

func (n *normalizer) normalizeTimes() {
	n.apply(timeWord, "time", func(sub []string) (string, string, string) {
		h, _ := strconv.Atoi(sub[1])
		m, _ := strconv.Atoi(sub[2])
		out := viNumberWords(uint64(h)) + " giờ"
		if m > 0 {
			out += " " + viNumberWords(uint64(m)) + " phút"
		}
		return out, sub[0], out
	})
	n.apply(timeColon, "time", func(sub []string) (string, string, string) {
		h, _ := strconv.Atoi(sub[1])
		m, _ := strconv.Atoi(sub[2])
		if h > 23 || m > 59 {
			return sub[0], "", ""
		}
		out := viNumberWords(uint64(h)) + " giờ"
		if m > 0 {
			out += " " + viNumberWords(uint64(m)) + " phút"
		}
		if s := sub[3]; s != "" {
			sec, _ := strconv.Atoi(s)
			if sec > 0 {
				out += " " + viNumberWords(uint64(sec)) + " giây"
			}
		}
		return out, sub[0], out
	})
	n.apply(timeH, "time", func(sub []string) (string, string, string) {
		h, _ := strconv.Atoi(sub[1])
		out := viNumberWords(uint64(h)) + " giờ"
		if sub[2] != "" {
			m, _ := strconv.Atoi(sub[2])
			if m > 0 {
				out += " " + viNumberWords(uint64(m)) + " phút"
			}
		}
		return out, sub[0], out
	})
}

var (
	// 金额单位同样要显式 Unicode 尾边界：ASCII 的 \b 会把 "5 điểm" 里的 "đ" 当单位。
	moneyDongRe = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)*)\s*(?:đồng|VNĐ|VND|vnđ|₫|đ)([^\p{L}\p{N}_]|$)`)
	moneyKRe    = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(?:k|nghìn)([^\p{L}\p{N}_]|$)`)
	moneyMRe    = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(?:tr|triệu)([^\p{L}\p{N}_]|$)`)
	moneyUSD    = regexp.MustCompile(`(?i)(?:\$\s*(\d+(?:[.,]\d+)*)|(\d+(?:[.,]\d+)*)\s*(?:USD|đô(?:\s+la)?))([^\p{L}\p{N}_]|$)`)
	moneyEUR    = regexp.MustCompile(`(?i)(?:€\s*(\d+(?:[.,]\d+)*)|(\d+(?:[.,]\d+)*)\s*(?:EUR|euro|€))([^\p{L}\p{N}_]|$)`)
	percentRe   = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*%`)
)

func (n *normalizer) normalizeMoney() {
	n.apply(moneyUSD, "money", func(sub []string) (string, string, string) {
		tok := sub[1]
		if tok == "" {
			tok = sub[2]
		}
		out := readNumberToken(tok) + " đô la" + sub[3]
		return out, sub[0], readNumberToken(tok) + " đô la"
	})
	n.apply(moneyEUR, "money", func(sub []string) (string, string, string) {
		tok := sub[1]
		if tok == "" {
			tok = sub[2]
		}
		out := readNumberToken(tok) + " euro" + sub[3]
		return out, sub[0], readNumberToken(tok) + " euro"
	})
	n.apply(moneyDongRe, "money", func(sub []string) (string, string, string) {
		out := readNumberToken(sub[1]) + " đồng" + sub[2]
		return out, sub[0], readNumberToken(sub[1]) + " đồng"
	})
	n.apply(moneyKRe, "money", func(sub []string) (string, string, string) {
		out := readNumberToken(sub[1]) + " nghìn" + sub[2]
		return out, sub[0], readNumberToken(sub[1]) + " nghìn"
	})
	n.apply(moneyMRe, "money", func(sub []string) (string, string, string) {
		out := readNumberToken(sub[1]) + " triệu" + sub[2]
		return out, sub[0], readNumberToken(sub[1]) + " triệu"
	})
}

func (n *normalizer) normalizePercent() {
	n.apply(percentRe, "percent", func(sub []string) (string, string, string) {
		out := readNumberToken(sub[1]) + " phần trăm"
		return out, sub[0], out
	})
}

// 斜杠读法：绝不对普通 "/" 读 "trên"。
//   - "và/hoặc"（含空格变体）→ "hoặc"
//   - 字母/字母 → "A hoặc B"
//   - 数字/数字：n<d 且 d≤10 才当分数读 "n trên d"（1/2, 3/4）；
//     其余（12/1, 28/10 之外的编号）用逗号停顿，不读 "trên"
//   - 其他 → 逗号停顿
var (
	slashAndOrRe = regexp.MustCompile(`(?i)\s*và\s*/\s*hoặc\s*`)
	slashNumRe   = regexp.MustCompile(`(\d+)\s*/\s*(\d+)`)
	slashWordRe  = regexp.MustCompile(`([\p{L}][\p{L}\p{N}_]*)\s*/\s*([\p{L}][\p{L}\p{N}_]*)`)
	slashAnyRe   = regexp.MustCompile(`\s*/\s*`)
)

func (n *normalizer) normalizeSlashes() {
	n.apply(slashAndOrRe, "symbol", func(sub []string) (string, string, string) {
		return " hoặc ", sub[0], "hoặc"
	})
	n.apply(slashNumRe, "symbol", func(sub []string) (string, string, string) {
		a, aErr := strconv.Atoi(sub[1])
		b, bErr := strconv.Atoi(sub[2])
		if aErr != nil || bErr != nil {
			return ", ", sub[0], ","
		}
		if a >= 1 && a < b && b <= 10 {
			out := readNumberToken(sub[1]) + " trên " + readNumberToken(sub[2])
			return out, sub[0], out
		}
		return sub[1] + ", " + sub[2], sub[0], sub[1] + ", " + sub[2]
	})
	n.apply(slashWordRe, "symbol", func(sub []string) (string, string, string) {
		out := sub[1] + " hoặc " + sub[2]
		return out, sub[0], out
	})
	n.apply(slashAnyRe, "symbol", func(sub []string) (string, string, string) {
		return ", ", sub[0], ","
	})
}

// viAbbreviations 是内置缩写表（项目可在 .ainovel/cach-doc.json 里补充/覆盖）。
var viAbbreviations = map[string]string{
	"TP.HCM": "thành phố Hồ Chí Minh",
	"TPHCM":  "thành phố Hồ Chí Minh",
	"HCM":    "Hồ Chí Minh",
	"HN":     "Hà Nội",
	"UBND":   "uỷ ban nhân dân",
	"HĐND":   "hội đồng nhân dân",
	"NXB":    "nhà xuất bản",
	"THPT":   "trung học phổ thông",
	"THCS":   "trung học cơ sở",
	"PGS":    "phó giáo sư",
	"GS":     "giáo sư",
	"TS":     "tiến sĩ",
	"BS":     "bác sĩ",
	"CLB":    "câu lạc bộ",
	"STT":    "số thứ tự",
	"VND":    "đồng",
	"USD":    "đô la",
	"TP":     "thành phố",
	"Q.":     "quận",
	"P.":     "phường",
	"Mr.":    "ông",
	"Mrs.":   "bà",
	"Ms.":    "cô",
	"Dr.":    "bác sĩ",
	"etc.":   "vân vân",
}

var abbreviationRes = buildAbbreviationRes()

func buildAbbreviationRes() []struct {
	re  *regexp.Regexp
	out string
} {
	keys := make([]string, 0, len(viAbbreviations))
	for k := range viAbbreviations {
		keys = append(keys, k)
	}
	// 长的优先，"TP.HCM" 先于 "TP"，避免 "TP.HCM" 被切成 "thành phố.HCM"。
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	out := make([]struct {
		re  *regexp.Regexp
		out string
	}, 0, len(keys))
	for _, k := range keys {
		re := regexp.MustCompile(`(^|[^\p{L}\p{N}_])(` + regexp.QuoteMeta(k) + `)([^\p{L}\p{N}_]|$)`)
		out = append(out, struct {
			re  *regexp.Regexp
			out string
		}{re, viAbbreviations[k]})
	}
	return out
}

func (n *normalizer) normalizeAbbreviations() {
	for _, ab := range abbreviationRes {
		re, out := ab.re, ab.out
		n.apply(re, "abbreviation", func(sub []string) (string, string, string) {
			return sub[1] + out + sub[3], sub[2], out
		})
	}
}

var (
	rangeRe  = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*[-–—]\s*(\d+(?:[.,]\d+)?)`)
	numberRe = regexp.MustCompile(`\d+(?:[.,]\d+)*`)
)

func (n *normalizer) normalizeRanges() {
	n.apply(rangeRe, "symbol", func(sub []string) (string, string, string) {
		out := readNumberToken(sub[1]) + " đến " + readNumberToken(sub[2])
		return out, sub[0], out
	})
}

func (n *normalizer) normalizeNumbers() {
	n.apply(ordinalRe, "ordinal", func(sub []string) (string, string, string) {
		v, err := strconv.ParseUint(sub[1], 10, 64)
		if err != nil {
			return sub[0], "", ""
		}
		out := "thứ " + viOrdinalWord(v)
		return out, sub[0], out
	})
	n.apply(numberRe, "number", func(sub []string) (string, string, string) {
		out := readNumberToken(sub[0])
		if out == sub[0] {
			return sub[0], "", ""
		}
		return out, sub[0], out
	})
}

// unitWords 的单位必须后面不是字母/数字：Go 的 \b 是 ASCII 边界，"5 mét" 里的
// "m" 会被当成单位（é 不算 word char），所以这里用显式 Unicode 边界并保留尾字符。
var unitWords = []struct {
	re  *regexp.Regexp
	out string
}{
	{unitRe(`km/h|kmh`), "ki-lô-mét trên giờ"},
	{unitRe(`km2|km²`), "ki-lô-mét vuông"},
	{unitRe(`m2|m²`), "mét vuông"},
	{unitRe(`m3|m³`), "mét khối"},
	{unitRe(`°C|độ C`), "độ xê"},
	{unitRe(`km`), "ki-lô-mét"},
	{unitRe(`kg`), "ki-lô-gam"},
	{unitRe(`cm`), "xen-ti-mét"},
	{unitRe(`mm`), "mi-li-mét"},
	{unitRe(`ml`), "mi-li-lít"},
	{unitRe(`GB`), "gi-ga-bai"},
	{unitRe(`MB`), "mê-ga-bai"},
	{unitRe(`KB`), "ki-lô-bai"},
	{unitRe(`kW`), "ki-lô-oát"},
	{unitRe(`W`), "oát"},
	{unitRe(`ha`), "héc-ta"},
	{unitRe(`m`), "mét"},
}

// unitRe 匹配"数字 + 单位 + 一个非字母数字的尾字符（或行尾）"。
func unitRe(unit string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(?:` + unit + `)([^\p{L}\p{N}_]|$)`)
}

func (n *normalizer) normalizeUnits() {
	for _, u := range unitWords {
		out := u.out
		n.apply(u.re, "unit", func(sub []string) (string, string, string) {
			rep := readNumberToken(sub[1]) + " " + out + sub[2]
			return rep, sub[0], readNumberToken(sub[1]) + " " + out
		})
	}
}

// looseUnitWords 处理"数字已经被区间读法吃掉"后残留的单位（"3-5 km" → "ba đến năm km"）。
// 单字母 "m" 不在此列：它太容易是缩写或代号。
var looseUnitWords = []struct {
	re  *regexp.Regexp
	out string
}{
	{regexp.MustCompile(`(?i)\b(?:km/h|kmh)\b`), "ki-lô-mét trên giờ"},
	{regexp.MustCompile(`(?i)\b(?:km2|km²)\b`), "ki-lô-mét vuông"},
	{regexp.MustCompile(`(?i)\b(?:m2|m²)\b`), "mét vuông"},
	{regexp.MustCompile(`(?i)\b(?:m3|m³)\b`), "mét khối"},
	{regexp.MustCompile(`(?i)\bkm\b`), "ki-lô-mét"},
	{regexp.MustCompile(`(?i)\bkg\b`), "ki-lô-gam"},
	{regexp.MustCompile(`(?i)\bcm\b`), "xen-ti-mét"},
	{regexp.MustCompile(`(?i)\bmm\b`), "mi-li-mét"},
	{regexp.MustCompile(`(?i)\bml\b`), "mi-li-lít"},
	{regexp.MustCompile(`(?i)\bGB\b`), "gi-ga-bai"},
	{regexp.MustCompile(`(?i)\bMB\b`), "mê-ga-bai"},
	{regexp.MustCompile(`(?i)\bKB\b`), "ki-lô-bai"},
	{regexp.MustCompile(`(?i)\bkW\b`), "ki-lô-oát"},
	{regexp.MustCompile(`(?i)\bha\b`), "héc-ta"},
}

func (n *normalizer) normalizeLooseUnits() {
	for _, u := range looseUnitWords {
		out := u.out
		n.apply(u.re, "unit", func(sub []string) (string, string, string) {
			return out, sub[0], out
		})
	}
}

func (n *normalizer) normalizeSymbols() {
	n.apply(regexp.MustCompile(`\s*&\s*`), "symbol", func(sub []string) (string, string, string) {
		return " và ", sub[0], "và"
	})
	n.apply(regexp.MustCompile(`\s*\+\s*`), "symbol", func(sub []string) (string, string, string) {
		return " cộng ", sub[0], "cộng"
	})
	n.apply(regexp.MustCompile(`\s*=\s*`), "symbol", func(sub []string) (string, string, string) {
		return " bằng ", sub[0], "bằng"
	})
	n.apply(regexp.MustCompile(`\s*×\s*`), "symbol", func(sub []string) (string, string, string) {
		return " nhân ", sub[0], "nhân"
	})
	n.apply(regexp.MustCompile(`\s*(?:→|->)\s*`), "symbol", func(sub []string) (string, string, string) {
		return " sang ", sub[0], "sang"
	})
	n.apply(regexp.MustCompile(`\s*°\s*`), "symbol", func(sub []string) (string, string, string) {
		return " độ ", sub[0], "độ"
	})
	n.apply(regexp.MustCompile(`\s*#\s*(\d+)`), "symbol", func(sub []string) (string, string, string) {
		return " số " + readNumberToken(sub[1]), sub[0], "số"
	})
}

// readNumberToken 读一个数字 token：千分位、小数、前导零（编号/电话）分别处理。
func readNumberToken(tok string) string {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return tok
	}
	// 千分位：1.234.567 / 1,234,567
	if viThousandsRe.MatchString(tok) || enThousandsRe.MatchString(tok) {
		digits := strings.NewReplacer(".", "", ",", "").Replace(tok)
		if v, err := strconv.ParseUint(digits, 10, 64); err == nil {
			return viNumberWords(v)
		}
	}
	// 小数：12,5（vi）/ 12.5（en，小数位 1–2 位，避免把 1.234 当小数）
	if m := viDecimalRe.FindStringSubmatch(tok); m != nil {
		return readDecimal(m[1], m[2])
	}
	if m := enDecimalRe.FindStringSubmatch(tok); m != nil {
		return readDecimal(m[1], m[2])
	}
	if strings.HasPrefix(tok, "0") && len(tok) > 1 {
		return viDigitsWords(tok)
	}
	v, err := strconv.ParseUint(tok, 10, 64)
	if err != nil {
		return viDigitsWords(tok)
	}
	return viNumberWords(v)
}

var (
	viThousandsRe = regexp.MustCompile(`^\d{1,3}(?:\.\d{3})+$`)
	enThousandsRe = regexp.MustCompile(`^\d{1,3}(?:,\d{3})+$`)
	viDecimalRe   = regexp.MustCompile(`^(\d+),(\d+)$`)
	enDecimalRe   = regexp.MustCompile(`^(\d+)\.(\d{1,2})$`)
)

// readDecimal 读小数："3,5" → "ba phẩy năm"；"0,05" → "không phẩy không năm"。
func readDecimal(intPart, fracPart string) string {
	fracPart = strings.TrimRight(fracPart, "0")
	if fracPart == "" {
		fracPart = "0"
	}
	var frac string
	if strings.HasPrefix(fracPart, "0") || len(fracPart) > 2 {
		frac = viDigitsWords(fracPart)
	} else {
		v, _ := strconv.ParseUint(fracPart, 10, 64)
		frac = viNumberWords(v)
	}
	return readNumberToken(intPart) + " phẩy " + frac
}

// countWords 按空白计词数（"Câu dài 41 từ" 的判定用）。
func countWords(s string) int { return len(strings.Fields(s)) }

var (
	// Từ ngữ cần xét là một cụm chữ Latin (kể cả chữ có dấu Việt/ngoại).
	// Trước đây regex chỉ ăn [A-Za-z] nên "toàn" bị chẻ thành "to" rồi cờ nhầm.
	englishTokenRe = regexp.MustCompile(`[\p{Latin}\p{Mn}\p{Mc}]{2,}`)
	englishSuffix  = regexp.MustCompile(`(?i)(?:ing|tion|sion|ed|ly|ous|ment|ness|able|ible|ical)$`)
	englishForbid  = "fjwz"
)

// englishWords 找出规范化之后仍然"像英文"的词，供 cờ "english"（Cần xem › từ tiếng Anh）。
// Lọc hai tầng: từ phải KHÔNG phải âm tiết Việt hợp lệ (isVietnameseSyllable),
// rồi mới xét chữ cấm f/j/w/z, hậu tố kiểu Anh (-ing, -tion…) hay bảng từ phổ biến.
// Vì vậy "in", "to", "can", "hot" — vừa là từ Anh vừa là âm tiết Việt — không bị cờ.
func englishWords(s string) []string {
	var out []string
	for _, tok := range englishTokenRe.FindAllString(s, -1) {
		if isVietnameseSyllable(tok) {
			continue
		}
		lower := strings.ToLower(tok)
		if commonEnglish[lower] {
			out = append(out, tok)
			continue
		}
		if strings.ContainsAny(lower, englishForbid) || englishSuffix.MatchString(lower) {
			out = append(out, tok)
		}
	}
	return out
}

var commonEnglish = map[string]bool{
	"the": true, "and": true, "you": true, "your": true, "of": true, "to": true, "in": true,
	"is": true, "are": true, "was": true, "were": true, "be": true, "been": true, "for": true,
	"on": true, "with": true, "at": true, "by": true, "from": true, "this": true, "that": true,
	"these": true, "those": true, "it": true, "its": true, "as": true, "but": true, "not": true,
	"have": true, "has": true, "had": true, "will": true, "would": true, "can": true, "could": true,
	"should": true, "do": true, "does": true, "did": true, "my": true, "his": true, "her": true,
	"our": true, "their": true, "me": true, "him": true, "them": true, "we": true, "they": true,
	"he": true, "she": true, "or": true, "if": true, "so": true, "up": true, "out": true,
	"about": true, "into": true, "over": true, "after": true, "before": true, "live": true,
	"show": true, "okay": true, "ok": true, "tv": true, "mc": true, "app": true, "video": true,
	"stream": true, "like": true, "comment": true, "share": true, "subscribe": true, "hello": true,
	"yes": true, "no": true, "please": true, "sorry": true, "thanks": true, "thank": true,
	// Từ phổ biến của thể loại/goi.json (10-audio §15.3) mà quy tắc hình dáng chưa bắt hết.
	"email": true, "mail": true, "scandal": true, "credit": true, "team": true, "post": true,
	"trend": true, "view": true, "story": true, "hashtag": true, "search": true, "fan": true,
	"fanpage": true, "spam": true, "hotline": true, "livestream": true, "streamer": true,
}
