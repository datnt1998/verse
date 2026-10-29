// Package naolint checks structured story facts without provider calls.
package naolint

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/rules"
)

type Finding struct {
	rules.Violation
	Chapter int    `json:"chapter"`
	Line    int    `json:"line,omitempty"`
	Quote   string `json:"quote,omitempty"`
}

// AddressRule là tên luật của kiểm xưng hô (khớp naolint.Chapter → rules.Violation.Rule).
const AddressRule = "address_terms"

// IssueRef là chip mục của thẻ thẩm duyệt cho vi phạm xưng hô (D4/D9).
const IssueRef = "Xưng hô"

// IssueName là tên mục khi dữ liệu lưu cũ không ghi chiều "A → B".
const IssueName = "Bảng xưng hô của truyện"

const quoteMark = "Trích: "

// Describe diễn đạt một vi phạm xưng hô đã lưu bằng một câu chỉ từ dữ liệu của nó:
// Target = "A → B", Actual = cách gọi đã dùng, Limit = cách gọi đúng.
func Describe(v rules.Violation) string {
	from, to, ok := strings.Cut(strings.TrimSpace(v.Target), "→")
	if !ok || strings.TrimSpace(v.Target) == "" {
		return MessageBody(v.Message)
	}
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	used := fmt.Sprint(v.Actual)
	if v.Limit == nil {
		return fmt.Sprintf("%s dùng “%s” khi nói với %s.", from, used, to)
	}
	return fmt.Sprintf("%s dùng “%s” thay vì “%s” khi nói với %s.", from, used, fmt.Sprint(v.Limit), to)
}

// Suggestion là chỉ dẫn cho người viết khi chương vào hàng đợi viết lại vì xưng
// hô: chỉ sửa đúng những câu dùng cách gọi đó, không viết lại cả chương.
func Suggestion(v rules.Violation) string {
	used := fmt.Sprint(v.Actual)
	if v.Limit == nil {
		return fmt.Sprintf("Chỉ sửa những câu dùng “%s”; giữ nguyên các câu còn lại của chương.", used)
	}
	return fmt.Sprintf("Chỉ sửa những câu dùng “%s” thành “%s”; giữ nguyên các câu còn lại của chương.", used, fmt.Sprint(v.Limit))
}

// MessageBody là phương án dự phòng khi không tách được chiều xưng hô: bỏ nhãn
// mức (UI không hiển thị "[LỖI]"/"[CẢNH BÁO]") và phần trích dẫn.
func MessageBody(message string) string {
	body := strings.TrimSpace(message)
	body = strings.TrimPrefix(body, "[LỖI] ")
	body = strings.TrimPrefix(body, "[CẢNH BÁO] ")
	if i := strings.LastIndex(body, quoteMark); i >= 0 {
		body = strings.TrimSpace(body[:i])
	}
	return body
}

// MessageQuote lấy đoạn trích ở cuối Message: rules.Violation không có trường
// quote nên Address ghi câu trích vào cuối thông điệp ("… Trích: <câu>").
func MessageQuote(message string) string {
	if i := strings.LastIndex(message, quoteMark); i >= 0 {
		return strings.TrimSpace(message[i+len(quoteMark):])
	}
	return ""
}

// Issue chuyển một vi phạm xưng hô đã lưu thành mục thẩm duyệt của vòng chấm
// (câu 13): mức error đi kèm requires_change để save_review đưa chương vào hàng
// đợi viết lại y như mọi mục Bắt sửa; mức warning chỉ là Cảnh báo.
func Issue(v rules.Violation, chapter int) domain.ConsistencyIssue {
	level := domain.SeverityWarning
	if v.Severity == rules.SeverityError {
		level = domain.SeverityError
	}
	quote := MessageQuote(v.Message)
	evidence := quote
	if strings.TrimSpace(evidence) == "" {
		evidence = Describe(v)
	}
	return domain.ConsistencyIssue{
		Type:           "address",
		Severity:       level,
		Description:    Describe(v),
		Evidence:       evidence,
		RuleRef:        IssueRef,
		RuleText:       strings.TrimSpace(v.Target),
		Quote:          quote,
		Suggestion:     Suggestion(v),
		Chapters:       []int{chapter},
		RequiresChange: domain.RequiresChangeFor(level, false),
	}
}

var quotes = regexp.MustCompile(`“([^”\n]+)”|"([^"\n]+)"`)
var words = regexp.MustCompile(`[\p{L}]+`)
var speech = regexp.MustCompile(`(?i)^(?:khẽ |lạnh lùng |nhẹ nhàng |vội |bỗng |lại |chỉ )*(?:nói|hỏi|đáp|trả lời|gọi|nhắn|thì thầm|quát|cười|tiếp lời|lên tiếng|bảo|gật đầu)(?:\b|[ :,.])`)
var pronouns = map[string]bool{"anh": true, "chị": true, "em": true, "cậu": true, "tớ": true, "tôi": true, "mình": true, "bác": true, "cô": true, "chú": true, "dì": true, "con": true, "cháu": true, "mày": true, "tao": true, "ông": true, "bà": true, "ta": true, "ngươi": true, "quý vị": true}

func containsName(text, name string) bool {
	if name == "" {
		return false
	}
	text = strings.ToLower(text)
	name = strings.ToLower(name)
	for start := 0; start < len(text); {
		i := strings.Index(text[start:], name)
		if i < 0 {
			return false
		}
		i += start
		end := i + len(name)
		previous, _ := utf8.DecodeLastRuneInString(text[:i])
		next, _ := utf8.DecodeRuneInString(text[end:])
		before := i == 0 || !unicode.IsLetter(previous)
		after := end == len(text) || !unicode.IsLetter(next)
		if before && after {
			return true
		}
		start = end
	}
	return false
}

func mentioned(text string, chars []domain.Character, narrator bool) []domain.Character {
	var out []domain.Character
	for _, c := range chars {
		for n := range len(c.Aliases) + 1 {
			name := c.Name
			if n > 0 {
				name = c.Aliases[n-1]
			}
			if pronouns[strings.ToLower(name)] && !(narrator && strings.EqualFold(name, "tôi")) {
				continue
			}
			if containsName(text, name) {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func tagged(text string, chars []domain.Character) (domain.Character, bool) {
	var found []domain.Character
	lower := strings.ToLower(text)
	for _, c := range chars {
		for n := range len(c.Aliases) + 1 {
			name := c.Name
			if n > 0 {
				name = c.Aliases[n-1]
			}
			if name == "" || pronouns[strings.ToLower(name)] && !strings.EqualFold(name, "tôi") {
				continue
			}
			n := strings.ToLower(name)
			i := strings.Index(lower, n)
			if i < 0 || !containsName(text, name) {
				continue
			}
			tail := strings.TrimSpace(lower[i+len(n):])
			if speech.MatchString(tail) {
				found = append(found, c)
				break
			}
		}
	}
	if len(found) == 1 {
		return found[0], true
	}
	return domain.Character{}, false
}

// Address checks quoted dialogue. Speech tags are certain; a sole named character
// in the same or an immediately adjacent narrative paragraph is inferred only.
func Address(chapter int, text string, chars []domain.Character, terms []domain.AddressTerm) []Finding {
	lines := strings.Split(text, "\n")
	var findings []Finding
	for i, line := range lines {
		spans := quotes.FindAllStringSubmatchIndex(line, -1)
		for _, span := range spans {
			start, end := span[2], span[3]
			if start < 0 {
				start, end = span[4], span[5]
			}
			quote := line[start:end]
			outside := quotes.ReplaceAllString(line, " ")
			speaker, certainSpeaker := tagged(outside, chars)
			context := outside
			if speaker.ID == "" {
				names := mentioned(outside, chars, true)
				if len(names) == 1 {
					speaker = names[0]
				}
			}
			if speaker.ID == "" {
				for _, step := range []int{-1, 1} {
					j := i + step
					for j >= 0 && j < len(lines) && strings.TrimSpace(lines[j]) == "" {
						j += step
					}
					if j < 0 || j >= len(lines) || quotes.MatchString(lines[j]) {
						continue
					}
					names := mentioned(lines[j], chars, true)
					if len(names) == 1 {
						speaker = names[0]
						context += " " + lines[j]
						break
					}
				}
			}
			if speaker.ID == "" {
				continue
			}
			var addressees []domain.Character
			// A name in a vocative at the beginning of a clause is certain, not a third-person mention.
			for _, c := range chars {
				if c.ID == speaker.ID {
					continue
				}
				for n := range len(c.Aliases) + 1 {
					name := c.Name
					if n > 0 {
						name = c.Aliases[n-1]
					}
					if pronouns[strings.ToLower(name)] {
						continue
					}
					q := strings.TrimSpace(quote)
					n := strings.ToLower(name)
					if strings.HasPrefix(strings.ToLower(q), n+",") || strings.HasPrefix(strings.ToLower(q), n+" à,") || strings.HasPrefix(strings.ToLower(q), n+" ơi,") {
						addressees = append(addressees, c)
						break
					}
				}
			}
			certainAddressee := len(addressees) == 1
			if len(addressees) == 0 {
				for _, c := range mentioned(context, chars, true) {
					if c.ID != speaker.ID {
						addressees = append(addressees, c)
					}
				}
			}
			if len(addressees) == 0 { // Only neighboring narrative paragraphs may identify the other participant.
				seen := map[string]bool{}
				for _, step := range []int{-1, 1} {
					j := i + step
					for j >= 0 && j < len(lines) && strings.TrimSpace(lines[j]) == "" {
						j += step
					}
					if j < 0 || j >= len(lines) || quotes.MatchString(lines[j]) {
						continue
					}
					if step > 0 {
						newParticipant := false
						for _, c := range mentioned(lines[j], chars, false) {
							if c.ID != speaker.ID {
								newParticipant = true
							}
						}
						if newParticipant {
							continue // A following paragraph can introduce a new scene, not an addressee.
						}
					}
					for _, c := range mentioned(lines[j], chars, true) {
						if c.ID != speaker.ID && !seen[c.ID] {
							seen[c.ID] = true
							addressees = append(addressees, c)
						}
					}
				}
			}
			if len(addressees) != 1 {
				continue
			}
			other := addressees[0]
			for _, term := range terms {
				if term.From != speaker.ID || term.To != other.ID || term.FromChapter > chapter || term.UntilChapter != 0 && term.UntilChapter < chapter {
					continue
				}
				for _, use := range pronounUses(quote, term) {
					expected := term.Calls
					if use.self {
						expected = term.Self
					}
					if strings.EqualFold(use.word, expected) {
						continue
					}
					severity := rules.SeverityWarning
					level := "[CẢNH BÁO]"
					if certainSpeaker && certainAddressee {
						severity = rules.SeverityError
						level = "[LỖI]"
					}
					role := "gọi"
					if use.self {
						role = "tự xưng"
					}
					findings = append(findings, Finding{Violation: rules.Violation{Rule: AddressRule, Target: speaker.Name + " → " + other.Name, Actual: use.word, Limit: expected, Severity: severity, Message: fmt.Sprintf("Master §15.3 %s chương %d, dòng %d: %s %s %q thay vì %q khi nói với %s. Phải sửa lỗi xưng hô; cảnh báo cần người duyệt xác nhận. %s%s", level, chapter, i+1, speaker.Name, role, use.word, expected, other.Name, quoteMark, quote)}, Chapter: chapter, Line: i + 1, Quote: quote})
				}
			}
		}
	}
	return findings
}

type pronounUse struct {
	word string
	self bool
}

func pronounUses(quote string, term domain.AddressTerm) []pronounUse {
	lower := strings.ToLower(quote)
	positions := words.FindAllStringIndex(quote, -1)
	var uses []pronounUse
	seen := map[string]bool{}
	explicitSelf := containsName(lower, "tôi") || containsName(lower, "tớ") || containsName(lower, "tao")
	for i, pos := range positions {
		word := strings.ToLower(quote[pos[0]:pos[1]])
		if !pronouns[word] {
			continue
		}
		if word == "mình" { // Usually inclusive “we”; not deterministically a self-pronoun.
			continue
		}
		next := ""
		if i+1 < len(positions) {
			next = strings.ToLower(quote[positions[i+1][0]:positions[i+1][1]])
			if word != "tôi" && word != "tớ" && word != "tao" {
				nextRune, _ := utf8.DecodeRuneInString(quote[positions[i+1][0]:positions[i+1][1]])
				if unicode.IsUpper(nextRune) {
					continue // “cô Tống”, “anh Phùng”: title + proper name may be third person.
				}
			}
		}
		if next == "ấy" || next == "ta" || next == "đó" || next == "này" && word == "cô" {
			continue
		}
		prefix := strings.ToLower(strings.TrimSpace(quote[:pos[0]]))
		clauseStart := prefix == "" || strings.ContainsAny(prefix[len(prefix)-1:], ".!?:;,")
		suffix := strings.ToLower(strings.TrimSpace(quote[pos[1]:]))
		vocative := clauseStart && (strings.HasPrefix(suffix, ",") || strings.HasPrefix(suffix, "ơi,") || strings.HasPrefix(suffix, "à,"))
		self := word == "tôi" || word == "tớ" || word == "tao"
		check := self || vocative
		if !check && clauseStart && containsName(lower, term.Calls) && word != strings.ToLower(term.Calls) && !explicitSelf {
			self = true
			check = true
		}
		if !check && explicitSelf && word != strings.ToLower(term.Self) && (clauseStart || strings.HasSuffix(prefix, "nhờ") || strings.HasSuffix(prefix, "cảm ơn") || strings.HasSuffix(prefix, "giúp") || strings.HasSuffix(prefix, "cho")) {
			check = true
		}
		if !check && clauseStart && (next == "đừng" || next == "hãy") {
			check = true
		}
		key := fmt.Sprintf("%t:%s", self, word)
		if check && !seen[key] {
			seen[key] = true
			uses = append(uses, pronounUse{word, self})
		}
	}
	return uses
}
