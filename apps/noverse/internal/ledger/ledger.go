// Package ledger owns the ten-axis story ledger and its distance checks.
package ledger

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/store"
)

// Names are verbatim from Hệ thống sản xuất liên tục §1.
var Axes = []Axis{{1, "Bộ quy tắc và biến thể"}, {2, "Bối cảnh"}, {3, "Nghề của nhân vật chính"}, {4, "Kiểu phản diện"}, {5, "Điểm thương"}, {6, "Vũ khí và cơ chế lật"}, {7, "Sân khấu cao trào"}, {8, "Tông giọng"}, {9, "Cấu trúc mở đầu"}, {10, "Kiểu kết"}}

type Axis struct {
	N    int    `json:"n"`
	Name string `json:"name"`
}
type Values [10]string
type Story struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Values Values `json:"values"`
	Day7   *int   `json:"day7"`
	// CreatedAt is the story's added date (story.json), drawn as the second line
	// of the ProjBrain ledger row (artboard: "24/10"). Null when the record has
	// no story.json yet — the row then draws the title alone.
	CreatedAt *time.Time `json:"createdAt"`
	Opening   string     `json:"opening"`
}
type Check struct {
	Level string `json:"level"`
	Rule  string `json:"rule"`
	Text  string `json:"text"`
	// Short is the one-line form of Text, built from the Đồng sáng tác templates
	// in the NewStory artboard. Empty only for records saved before it existed.
	Short string `json:"short"`
	// Fix is the right-aligned suggestion in the ProjBrain ledger card. Null when
	// the check passes or the artboard draws no suggestion for that rule (Câu 3).
	Fix     *string `json:"fix"`
	Axes    []int   `json:"axes"`
	Against string  `json:"against,omitempty"`
}

// axisShort are the matrix-header labels the UI uses (i18n vi.ts › axisNames,
// artboard AX): checks[].short must read like the artboard, so it names an axis
// the way the ledger table does.
var axisShort = [10]string{"Bộ & biến thể", "Bối cảnh", "Nghề", "Phản diện", "Điểm thương", "Vũ khí", "Sân khấu", "Tông", "Mở đầu", "Kết"}

// fixText is the ProjBrain artboard's right-aligned suggestion. A passed check
// carries null, as does every rule/level the artboard draws without one; the
// artboard only shows suggestions for a duplicate combination and overuse.
func fixText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type Cell struct {
	Story string `json:"story"`
	Axis  int    `json:"axis"`
	Mark  string `json:"mark"`
}
type Result struct {
	Axes   []Axis  `json:"axes"`
	Values Values  `json:"values"`
	Checks []Check `json:"checks"`
	Cells  []Cell  `json:"cells"`
}
type Record struct {
	Values Values `json:"values"`
	Brief  string `json:"brief"`
	Result Result `json:"result"`
}

const recordPath = "meta/ledger.json"

// Load reads the ledger from the file backend directory.
func Load(dir string) (*Record, error) { return LoadFrom(store.NewIO(dir)) }

// LoadFrom reads the ledger through a store document surface (file: the engine
// directory; postgres: story_state rows). Missing is (nil, nil).
func LoadFrom(io *store.IO) (*Record, error) {
	var r Record
	err := io.ReadJSON(recordPath, &r)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// Save writes the ledger to the file backend directory.
func Save(dir string, r Record) error { return SaveTo(store.NewIO(dir), r) }

// SaveTo writes the ledger through a store document surface.
func SaveTo(io *store.IO, r Record) error { return io.WriteJSON(recordPath, r) }
func Validate(v Values) error {
	for i, s := range v {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("Chưa xác định trục %d: %s", i+1, Axes[i].Name)
		}
	}
	return nil
}
func equal(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}
func (r Result) Blocking() bool {
	for _, c := range r.Checks {
		if c.Level == "error" {
			return true
		}
	}
	return false
}

// CheckValues takes prior stories newest first, never including the candidate.
// Four checks follow D14/artboard; overuse is limited to axes 3, 4 and 7.
func CheckValues(v Values, prior []Story) Result {
	if len(prior) > 10 {
		prior = prior[:10]
	}
	r := Result{Axes: Axes, Values: v, Checks: make([]Check, 0, 4), Cells: []Cell{}}
	combo := Check{Level: "ok", Rule: "combination", Text: "Tổ hợp trục 1 + 3 + 4 + 6 chưa dùng trong 10 truyện gần nhất.", Short: "Bốn trục chính chưa trùng truyện nào.", Axes: []int{1, 3, 4, 6}}
	for _, s := range prior {
		match := true
		for _, n := range combo.Axes {
			if !equal(v[n-1], s.Values[n-1]) || v[n-1] == "" {
				match = false
			}
		}
		if match {
			combo.Level = "error"
			combo.Against = s.ID
			combo.Text = fmt.Sprintf("Trùng tổ hợp trục 1 + 3 + 4 + 6 với “%s”.", s.Title)
			combo.Short = fmt.Sprintf("Giống “%s” ở 4 trục chính (bộ, nghề, phản diện, vũ khí).", s.Title)
			combo.Fix = fixText("Đổi ít nhất 1 trong 4 trục")
			for _, n := range combo.Axes {
				r.Cells = append(r.Cells, Cell{"new", n, "dup"}, Cell{s.ID, n, "dup"})
			}
			break
		}
	}
	r.Checks = append(r.Checks, combo)
	distance := Check{Level: "ok", Rule: "distance", Text: "Chưa có truyện liền trước để so khoảng cách.", Short: "Chưa có truyện liền trước để so khoảng cách.", Axes: []int{}}
	if len(prior) > 0 && Validate(prior[0].Values) == nil {
		for i, x := range v {
			if !equal(x, prior[0].Values[i]) {
				distance.Axes = append(distance.Axes, i+1)
			}
		}
		distance.Against = prior[0].ID
		distance.Text = fmt.Sprintf("Khác truyện liền trước ở %d trục (cần ít nhất 4).", len(distance.Axes))
		distance.Short = fmt.Sprintf("Khác truyện vừa đăng %d/10 trục.", len(distance.Axes))
		if len(distance.Axes) < 4 {
			distance.Level = "error"
			distance.Short = fmt.Sprintf("Chỉ khác truyện vừa đăng %d/10 trục.", len(distance.Axes))
		}
	}
	if len(prior) > 0 && Validate(prior[0].Values) != nil {
		distance.Level = "warning"
		distance.Against = prior[0].ID
		distance.Text = "Truyện liền trước chưa có đủ mười trục để so khoảng cách."
		distance.Short = distance.Text
	}
	r.Checks = append(r.Checks, distance)
	over := Check{Level: "ok", Rule: "overuse", Text: "Nghề, kiểu phản diện và sân khấu chưa vượt 2 lần trong 10 truyện.", Short: "Không trục nào dùng quá 2 lần.", Axes: []int{}}
	messages := []string{}
	shorts := []string{}
	for _, n := range []int{3, 4, 7} {
		count := 0
		for _, s := range prior {
			if v[n-1] != "" && equal(v[n-1], s.Values[n-1]) {
				count++
			}
		}
		if count >= 2 {
			over.Level = "warning"
			over.Fix = fixText("Cân nhắc đổi")
			over.Axes = append(over.Axes, n)
			messages = append(messages, fmt.Sprintf("%s “%s” đã dùng %d lần; truyện mới sẽ vượt tối đa 2 lần.", Axes[n-1].Name, v[n-1], count))
			shorts = append(shorts, fmt.Sprintf("%s “%s” đã có %d truyện dùng", axisShort[n-1], v[n-1], count))
			r.Cells = append(r.Cells, Cell{"new", n, "overused"})
			for _, s := range prior {
				if equal(v[n-1], s.Values[n-1]) {
					r.Cells = append(r.Cells, Cell{s.ID, n, "overused"})
				}
			}
		}
	}
	if len(messages) > 0 {
		over.Text = strings.Join(messages, " ")
		// One row for every overused axis (Câu 1), so the axes are joined, not split.
		over.Short = strings.Join(shorts, " · ") + "."
	}
	r.Checks = append(r.Checks, over)
	opening := Check{Level: "ok", Rule: "opening", Text: "Kiểu mở đầu không lặp 3 truyện liền nhau.", Short: "Mở đầu khác 2 truyện trước.", Axes: []int{9}}
	if len(prior) >= 2 && v[8] != "" && equal(v[8], prior[0].Values[8]) && equal(v[8], prior[1].Values[8]) {
		opening.Level = "warning"
		opening.Text = fmt.Sprintf("Mở đầu “%s” sẽ thành 3 truyện liền nhau.", v[8])
		opening.Short = "Mở đầu lặp 3 truyện liền."
		r.Cells = append(r.Cells, Cell{"new", 9, "overused"})
	}
	r.Checks = append(r.Checks, opening)
	return r
}
