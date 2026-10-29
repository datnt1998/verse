// Package stylestat 对已写正文做全书级风格统计，产出纯事实。
//
// 动机：弧内评审窗口（~10 章）对全书级模式固化天然失明——句式 tic 章均几十次、
// 章末形态同构、跨章复读，单章看每处都"正常"，只有全书统计能暴露。统计归代码
// （确定性、零幻觉），裁定归 LLM（editor 按数字判维度分，writer 据此自避免）。
// Compute 供离线评测一次性全量计算；运行时使用 Tracker 按章节增量维护。
package stylestat

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// minChapters 少于此章数不出统计——样本太小，频率没有意义。
const minChapters = 5

// phraseWindow 动态短语挖掘只看最近 N 章：writer 需要避免的是"现在的口头禅"。
const phraseWindow = 20

// Input 统计输入。Chapters 按章号升序；Stopwords 为角色名等专有名词，
// 动态短语挖掘时跳过（出场人名天然高频，不是文风问题）。
type Input struct {
	Chapters  []string
	Titles    []string
	Stopwords []string
}

// Stats 全书风格统计结果。所有字段都是事实计数，不含任何裁定或指令。
type Stats struct {
	Chapters          int            `json:"chapters"`
	Patterns          []PatternStat  `json:"patterns,omitempty"`
	TopPhrases        []PhraseStat   `json:"top_phrases,omitempty"`
	RepeatedSentences []SentenceStat `json:"repeated_sentences,omitempty"`
	Ending            EndingStat     `json:"ending"`
	OpeningTimeRate   float64        `json:"opening_time_rate"`
	TitleFormats      *TitleStat     `json:"title_formats,omitempty"`
}

// PatternStat 固定句式模式类的全书计数（通用 AI 文风 tic）。
type PatternStat struct {
	Name       string  `json:"name"`
	Total      int     `json:"total"`
	PerChapter float64 `json:"per_chapter"`
}

// PhraseStat 最近 phraseWindow 章内挖掘出的高频短语。
type PhraseStat struct {
	Text  string `json:"text"`
	Count int    `json:"count"`
}

// SentenceStat 跨章逐字重复的长句（复读交代的直接证据）。
type SentenceStat struct {
	Text     string `json:"text"`
	Chapters int    `json:"chapters"`
	Count    int    `json:"count"`
}

// EndingStat 章末行形态分布。短结尾本身合法，全书同构才是问题。
// 长度单位是词；JSON 字段名沿用 median_runes（模型侧读取的既有字段名，
// 不随语义改动改名），值自本次起按词计。
type EndingStat struct {
	ShortRatio  float64 `json:"short_ratio"`
	MedianWords int     `json:"median_runes"`
}

// TitleStat 章节标题「第N章」前缀混用计数（混用=机制痕迹暴露在产物里）。
type TitleStat struct {
	WithPrefix    int `json:"with_prefix"`
	WithoutPrefix int `json:"without_prefix"`
}

// patternDefs 通用 AI 文风句式模式。计数是近似（正则不做语法分析），
// 用途是本书自身的纵向基线对比，绝对精度不重要。
// 越南语专用：条目取自 assets/references/anti-ai-tone.md 的越南语清单。
var patternDefs = []struct {
	name string
	re   *regexp.Regexp
}{
	{"Câu đối lập“không phải… mà là”", regexp.MustCompile(`(?i)không phải[^.!?\n]{1,60}?mà (?:là|chính là)`)},
	{"Lượng từ thời gian“một thoáng/chốc lát”", regexp.MustCompile(`(?i)một thoáng|chốc lát|trong chớp mắt|một lát|giây lát`)},
	{"So sánh sáo“như thể/tựa như/giống như”", regexp.MustCompile(`(?i)như thể|tựa như|tựa hồ|giống như|như là|hệt như`)},
	{"Nhịp im lặng“im lặng/không nói gì”", regexp.MustCompile(`(?i)im lặng|không nói gì|không đáp|lặng thinh|không quay đầu`)},
	{"Mẫu thần thái“ánh mắt lóe lên/khoé môi nhếch”", regexp.MustCompile(`(?i)ánh mắt (?:lóe|loé) lên|khoé (?:môi|miệng)[^.!?\n]{0,16}?(?:nhếch|giật)|cắn môi|đồng tử co lại|viền mắt đỏ|không thể tin nổi`)},
	{"Phản ứng cơ thể“tim thắt lại/người run lên”", regexp.MustCompile(`(?i)tim[^.!?\n]{0,16}?(?:thắt|nhói) lại|người run lên|toàn thân (?:cứng|run)|hít một hơi lạnh|sống lưng lạnh|lạnh sống lưng`)},
	{"Dấu hiệu suy nghĩ“nghĩ thầm/nhận ra”", regexp.MustCompile(`(?i)nghĩ thầm|thầm nghĩ|nhận ra rằng|chợt hiểu|cảm thấy`)},
	{"Sáo trừu tượng“một cảm giác khó tả”", regexp.MustCompile(`(?i)một (?:cảm giác|nỗi) (?:khó tả|không tên)|không thể diễn tả|ý nghĩa (?:thật sự )?của|điều (?:thật sự|quan trọng nhất) (?:là|chính là)`)},
}

var (
	sentenceSplit = regexp.MustCompile(`[.!?…\n]+`)
	openingTimeRe = regexp.MustCompile(`(?i)(?:^|[^\p{L}])(?:đêm|cả đêm|bình minh|rạng đông|tảng sáng|hừng sáng|sáng sớm|buổi sáng|trời sáng|thức giấc|tỉnh dậy|ánh ban mai|hoàng hôn)(?:[^\p{L}]|$)`)
	// 标题前缀混用统计：「Chương N」是当前写法，`第N章`/`Chapter N` 保留给旧数据与英文标题。
	// 中文数字（第一章/第十二章）必须一并认：旧库标题不能因语种切换被判成"无前缀"。
	titlePrefixRe = regexp.MustCompile(`(?i)^#{0,2}\s*(?:chương\s*\d+|chapter\s*\d+|第\s*[零〇一二三四五六七八九十百千万\d]+\s*章)`)
)

// shortEndingWords 末行不超过此词数计为"短结尾"。
// 旧值 30 rune（30 个汉字 ≈ 30 音节）按 1 汉字 ≈ 1 越南语词换算仍是 30 词。
const shortEndingWords = 30

// Compute 计算全书风格统计；章数不足时返回 nil。
func Compute(in Input) *Stats {
	n := len(in.Chapters)
	if n < minChapters {
		return nil
	}
	all := strings.Join(in.Chapters, "\n")

	s := &Stats{Chapters: n}
	for _, def := range patternDefs {
		total := len(def.re.FindAllStringIndex(all, -1))
		if total == 0 {
			continue
		}
		s.Patterns = append(s.Patterns, PatternStat{
			Name:       def.name,
			Total:      total,
			PerChapter: round1(float64(total) / float64(n)),
		})
	}
	s.TopPhrases = minePhrases(recentWindow(in.Chapters), in.Stopwords)
	s.RepeatedSentences = repeatedSentences(in.Chapters)
	s.Ending = endingShape(in.Chapters)
	s.OpeningTimeRate = openingTimeRate(in.Chapters)
	s.TitleFormats = titleFormats(in.Titles)
	return s
}

func recentWindow(chapters []string) []string {
	if len(chapters) <= phraseWindow {
		return chapters
	}
	return chapters[len(chapters)-phraseWindow:]
}

// minePhrases 在窗口内挖掘 2-4 词的高频短语。
//
// 旧实现按 3-6 个汉字切 n-gram；越南语一个音节就是一个空白分隔词，故按词切
// 2-4 词（≈ 2-4 音节，如 "trong chớp mắt" 4 词），语义颗粒与旧实现等价。
// 过滤：非纯字母词、首尾虚词、命中专有名词；去重：与已选短语互为子串的丢弃。
func minePhrases(chapters []string, stopwords []string) []PhraseStat {
	words := tokenize(strings.Join(chapters, "\n"))
	threshold := max(8, len(chapters)/2)

	counts := make(map[string]int)
	for size := 2; size <= 4; size++ {
		for i := 0; i+size <= len(words); i++ {
			gram := words[i : i+size]
			if !validGram(gram) {
				continue
			}
			counts[strings.Join(gram, " ")]++
		}
	}

	stopGrams := stopwordGrams(stopwords)
	type cand struct {
		text  string
		count int
	}
	var cands []cand
	for g, c := range counts {
		if c < threshold || hitStopword(strings.Split(g, " "), stopGrams) {
			continue
		}
		cands = append(cands, cand{g, c})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].count != cands[j].count {
			return cands[i].count > cands[j].count
		}
		// 同频取更长的（信息量更大），再按字典序稳定排序
		if len(cands[i].text) != len(cands[j].text) {
			return len(cands[i].text) > len(cands[j].text)
		}
		return cands[i].text < cands[j].text
	})

	var out []PhraseStat
	for _, c := range cands {
		if len(out) >= 8 {
			break
		}
		dup := false
		for _, picked := range out {
			if strings.Contains(picked.Text, c.text) || strings.Contains(c.text, picked.Text) {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, PhraseStat{Text: c.text, Count: c.count})
		}
	}
	return out
}

// gramEdgeStop 首尾为这些虚词/代词的词不是文风短语，跳过（旧实现是单字虚词表，
// 越南语按词对齐后同一批功能词扩成词表）。
var gramEdgeStop = map[string]struct{}{
	"của": {}, "và": {}, "là": {}, "có": {}, "không": {}, "một": {}, "những": {}, "các": {},
	"đã": {}, "đang": {}, "sẽ": {}, "thì": {}, "mà": {}, "để": {}, "với": {}, "trong": {},
	"ra": {}, "vào": {}, "lên": {}, "xuống": {}, "rồi": {}, "như": {}, "cũng": {}, "nhưng": {},
	"nên": {}, "vì": {}, "ở": {}, "từ": {}, "đến": {}, "cho": {}, "về": {}, "khi": {},
	"nếu": {}, "này": {}, "đó": {}, "kia": {}, "ấy": {}, "tôi": {}, "nó": {}, "hắn": {},
	"nàng": {}, "y": {}, "thị": {}, "anh": {}, "cô": {}, "ông": {}, "bà": {}, "chúng": {},
	"họ": {}, "mình": {}, "ai": {}, "gì": {}, "sao": {}, "thế": {}, "vậy": {}, "còn": {},
	"phải": {}, "được": {}, "bị": {}, "do": {}, "bằng": {}, "theo": {}, "qua": {}, "sau": {},
	"trước": {}, "khiến": {}, "rằng": {},
}

func validGram(gram []string) bool {
	if _, ok := gramEdgeStop[gram[0]]; ok {
		return false
	}
	if _, ok := gramEdgeStop[gram[len(gram)-1]]; ok {
		return false
	}
	return true
}

// tokenize 把正文切成小写词：按空白分隔，剥掉两侧标点，丢弃不含字母的词
// （数字、破折号、省略号都不是短语成分）。
func tokenize(text string) []string {
	var out []string
	for _, raw := range strings.Fields(text) {
		tok := strings.ToLower(strings.TrimFunc(raw, func(r rune) bool {
			return !unicode.IsLetter(r)
		}))
		if tok == "" || !isLetters(tok) {
			continue
		}
		out = append(out, tok)
	}
	return out
}

func isLetters(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// stopwordGrams 把专有名词拆成词片段：人名常以部分形式入文（"Cửu Uyên đứng dậy"
// 含 "Cửu Uyên"），按整名匹配会漏网。宁可过滤偏严——短语事实少一条无碍，
// 人名混进口头禅清单才是噪声。越南语姓名逐词都是独立词，故多词姓名除整名与
// 相邻两词窗口外，每个词也单独作为过滤片段。
func stopwordGrams(stopwords []string) [][]string {
	var grams [][]string
	for _, w := range stopwords {
		words := tokenize(w)
		if len(words) == 0 {
			continue
		}
		grams = append(grams, words)
		for _, word := range words {
			grams = append(grams, []string{word})
		}
		for i := 0; i+2 <= len(words); i++ {
			grams = append(grams, words[i:i+2])
		}
	}
	return grams
}

// hitStopword 判断 n-gram 里是否连续出现某个专有名词片段。
func hitStopword(gram []string, stopGrams [][]string) bool {
	for _, stop := range stopGrams {
		for i := 0; i+len(stop) <= len(gram); i++ {
			match := true
			for j := range stop {
				if gram[i+j] != stop[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// repeatedSentences 找跨 ≥3 章逐字重复的 ≥12 词句子，按次数取 top 5。
func repeatedSentences(chapters []string) []SentenceStat {
	type rec struct {
		count    int
		chapters map[int]struct{}
	}
	seen := make(map[string]*rec)
	for ci, text := range chapters {
		for sent, count := range chapterSentenceCounts(text) {
			r := seen[sent]
			if r == nil {
				r = &rec{chapters: make(map[int]struct{})}
				seen[sent] = r
			}
			r.count += count
			r.chapters[ci] = struct{}{}
		}
	}

	var out []SentenceStat
	for sent, r := range seen {
		if len(r.chapters) < 3 {
			continue
		}
		out = append(out, SentenceStat{Text: truncateWords(sent, 40), Chapters: len(r.chapters), Count: r.count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Text < out[j].Text
	})
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

// trimWrappedQuotes 剥掉包裹引号：同一句台词带/不带前引号不应算成两条。
func trimWrappedQuotes(sentence string) string {
	return strings.Trim(strings.TrimSpace(sentence), `"“”‘’「」『』`)
}

func endingShape(chapters []string) EndingStat {
	var lengths []int
	short := 0
	for _, text := range chapters {
		line := lastNonEmptyLine(text)
		if line == "" {
			continue
		}
		n := domain.WordCount(line)
		lengths = append(lengths, n)
		if n <= shortEndingWords {
			short++
		}
	}
	if len(lengths) == 0 {
		return EndingStat{}
	}
	sort.Ints(lengths)
	return EndingStat{
		ShortRatio:  round2(float64(short) / float64(len(lengths))),
		MedianWords: lengths[len(lengths)/2],
	}
}

func openingTimeRate(chapters []string) float64 {
	hit := 0
	for _, text := range chapters {
		if openingTimeRe.MatchString(firstParagraph(text)) {
			hit++
		}
	}
	return round2(float64(hit) / float64(len(chapters)))
}

func titleFormats(titles []string) *TitleStat {
	if len(titles) == 0 {
		return nil
	}
	t := &TitleStat{}
	for _, title := range titles {
		if strings.TrimSpace(title) == "" {
			continue
		}
		if titlePrefixRe.MatchString(title) {
			t.WithPrefix++
		} else {
			t.WithoutPrefix++
		}
	}
	// 只有混用才值得上报；统一格式不是事实意义上的问题
	if t.WithPrefix == 0 || t.WithoutPrefix == 0 {
		return nil
	}
	return t
}

func lastNonEmptyLine(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

// firstParagraph 取第一个非空且非 Markdown 标题的行（章文件首行常是 # 标题）。
func firstParagraph(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line
	}
	return ""
}

func truncateWords(s string, n int) string {
	fields := strings.Fields(s)
	if len(fields) <= n {
		return s
	}
	return strings.Join(fields[:n], " ") + "…"
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
