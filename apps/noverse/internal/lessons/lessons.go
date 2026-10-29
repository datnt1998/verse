// Package lessons learns only rule-level patterns; story prose never crosses its interface.
package lessons

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	CandidateStories = 2
	AlertStories     = 3
)

// Observation contains trusted rule metadata, not issue text, fixes or quotes.
type Observation struct {
	ID, Story, StoryTitle, PackID, PackName, Ref, RefName, Level string
	Chapter                                                      int
	At                                                           time.Time
}

// Evidence counts each source kind separately (§M): Bắt sửa, Cảnh báo and the
// corrections a person typed. `object` is the noun a user lesson carries
// ("xưng hô" in "3 lần bạn sửa xưng hô") — today only legacy files supply it.
type Evidence struct {
	Errors    int    `json:"errors"`
	Warnings  int    `json:"warnings"`
	UserEdits int    `json:"userEdits"`
	Object    string `json:"object,omitempty"`
	Stories   int    `json:"stories"`
	// Pack chỉ có tên gói với mục `Bộ …`; mục dùng chung luôn "" và dòng nguồn ghi “· N gói”
	// (kể cả “· 1 gói”), nên FE không bao giờ đọc tên gói từ đây cho mục dùng chung.
	Pack       string   `json:"pack"`
	PacksCount int      `json:"packsCount"`
	Chips      []string `json:"chips"`

	// legacyKind giữ khoá "kind" của bản ghi cũ: cần để dựng lại id tách-mức
	// trong migrate; không ghi lại ra file (dữ liệu cũ không chuyển — §M).
	legacyKind string
}

// legacyEvidence là hình dáng evidence trước §M: một `count` duy nhất và `kind`
// vừa là mức (error|warning) vừa là danh từ người dùng sửa.
type legacyEvidence struct {
	Count int    `json:"count"`
	Kind  string `json:"kind"`
}

// UnmarshalJSON đọc được cả bản ghi cũ: `kind` là mức thì `count` thành
// errors/warnings, `kind` là danh từ thì `count` thành userEdits + object.
// Không ghi lại file — chỉ suy ra khi đọc (§M). Trường mới đã có thì giữ nguyên.
func (e *Evidence) UnmarshalJSON(data []byte) error {
	type flat Evidence
	var v struct {
		flat
		legacyEvidence
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*e = Evidence(v.flat)
	e.legacyKind = v.Kind
	if v.Kind == "" || e.Errors != 0 || e.Warnings != 0 || e.UserEdits != 0 {
		return nil
	}
	switch v.Kind {
	case "error":
		e.Errors = v.Count
	case "warning":
		e.Warnings = v.Count
	default:
		e.UserEdits = v.Count
		e.Object = v.Kind
	}
	return nil
}

// equal so sánh theo giá trị phần FE đọc được; legacyKind chỉ phục vụ migrate nên
// không quyết định Sync có phải làm mới hay không.
func (e Evidence) equal(o Evidence) bool {
	return e.Errors == o.Errors && e.Warnings == o.Warnings && e.UserEdits == o.UserEdits &&
		e.Object == o.Object && e.Stories == o.Stories && e.Pack == o.Pack &&
		e.PacksCount == o.PacksCount && slices.Equal(e.Chips, o.Chips)
}

type Lesson struct {
	ID        string     `json:"id"`
	Origin    string     `json:"origin"`
	Refs      []string   `json:"refs"`
	Text      string     `json:"text"`
	Evidence  Evidence   `json:"evidence"`
	Scope     string     `json:"scope"`
	PackID    string     `json:"packId"`
	PackName  string     `json:"packName"`
	State     string     `json:"state"`
	CreatedAt time.Time  `json:"createdAt"`
	DecidedAt *time.Time `json:"decidedAt,omitempty"`
}

type Row struct {
	Ref     string `json:"ref"`
	RefName string `json:"refName"`
	Stories int    `json:"stories"`
	Total   int    `json:"total"`
	Level   string `json:"level"`
	Alert   bool   `json:"alert"`
	// Mục dùng chung mang chuông tính trên mọi gói (câu 10): `alertScope` luôn có, `alertStories`
	// luôn có kể cả 0 để FE không phải đoán. Mục `Bộ …` bỏ trống cả hai và giữ chuông theo gói.
	AlertScope   string `json:"alertScope,omitempty"`
	AlertStories *int   `json:"alertStories,omitempty"`
}

// packRefPattern nhận mục luật riêng của một gói (`Bộ A §4.3`, `Bộ C §12`). Các nhãn
// còn lại là lớp dùng chung hoặc luật của người dùng: `Master §7`, `Định dạng §3`,
// `Xưng hô`, `Quy tắc của bạn`.
var packRefPattern = regexp.MustCompile(`^Bộ\s+\S+\s*§`)

// IsPackRef báo mục luật chỉ thuộc một gói; mục không thuộc gói nào được nhóm qua mọi gói (câu 15).
func IsPackRef(ref string) bool { return packRefPattern.MatchString(strings.TrimSpace(ref)) }

// GroupKey là khoá nhóm ổn định: (gói, mục) cho mục riêng của gói, chỉ (mục) cho mục dùng chung.
func GroupKey(packID, ref string) string {
	if IsPackRef(ref) {
		return packID + "\x00" + ref
	}
	return "all\x00" + ref
}

// ID băm khoá nhóm; cùng một nhóm luôn cùng id qua mọi lần chấm.
func ID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "lesson-" + hex.EncodeToString(sum[:12])
}

type group struct {
	first        Observation
	errors       int
	warnings     int
	stories      map[string]bool
	errorStories map[string]bool
	packStories  map[string]map[string]bool
	packErrors   map[string]map[string]bool
	chips        map[string]bool
	at           time.Time
}

func groups(observations []Observation) map[string]*group {
	out := map[string]*group{}
	seen := map[string]bool{}
	for _, o := range observations {
		if o.Ref == "" || o.RefName == "" || o.PackID == "" {
			continue
		}
		key := GroupKey(o.PackID, o.Ref)
		unique := key + "\x00" + o.Story + "\x00" + o.ID
		if seen[unique] {
			continue
		}
		seen[unique] = true
		g := out[key]
		if g == nil {
			g = &group{first: o, stories: map[string]bool{}, errorStories: map[string]bool{},
				packStories: map[string]map[string]bool{}, packErrors: map[string]map[string]bool{}, chips: map[string]bool{}}
			out[key] = g
		}
		g.stories[o.Story] = true
		if g.packStories[o.PackID] == nil {
			g.packStories[o.PackID] = map[string]bool{}
			g.packErrors[o.PackID] = map[string]bool{}
		}
		g.packStories[o.PackID][o.Story] = true
		if o.Level == "error" {
			g.errors++
			g.errorStories[o.Story] = true
			g.packErrors[o.PackID][o.Story] = true
		} else {
			g.warnings++
		}
		g.chips[fmt.Sprintf("%s · ch.%d", o.StoryTitle, o.Chapter)] = true
		if !o.At.IsZero() && (g.at.IsZero() || o.At.Before(g.at)) {
			g.at = o.At
		}
	}
	return out
}

// Aggregate proposes a stable lesson for each rule section recurring across stories.
// Mức không bao giờ tách nhóm: bằng chứng đếm cả Bắt sửa lẫn Cảnh báo, mức bài học là mức cao nhất.
// Only titles supplied by the pack are used: review quotes may contain private plots.
func Aggregate(observations []Observation) []Lesson {
	out := []Lesson{}
	for key, g := range groups(observations) {
		if len(g.stories) < CandidateStories {
			continue
		}
		chips := make([]string, 0, len(g.chips))
		for c := range g.chips {
			chips = append(chips, c)
		}
		sort.Strings(chips)
		// Mục riêng của gói ở lại trong gói và mang tên gói. Mục dùng chung luôn ghi “· N gói”
		// (kể cả “· 1 gói”), nên không mang tên gói nào (câu 4).
		scope, packID, packName, packLabel, packsCount := "all", g.first.PackID, g.first.PackName, "", len(g.packStories)
		if IsPackRef(g.first.Ref) {
			scope, packLabel, packsCount = "pack", g.first.PackName, 1
		}
		out = append(out, Lesson{ID: ID(key), Origin: "review", Refs: []string{g.first.Ref},
			Text:     fmt.Sprintf("Khi lập đại cương, kiểm tra mục %s — %s để không lặp vi phạm ở truyện mới.", g.first.Ref, g.first.RefName),
			Evidence: Evidence{Errors: g.errors, Warnings: g.warnings, Stories: len(g.stories), Pack: packLabel, PacksCount: packsCount, Chips: chips},
			Scope:    scope, PackID: packID, PackName: packName, State: "pending", CreatedAt: g.at})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Violations counts distinct stories, including only error stories for the alert.
// Bảng là “theo gói”, nên số truyện và mức vẫn tính trong đúng gói đang xem. Chuông của mục
// dùng chung tính trên mọi gói (câu 10) và mang thêm `alertScope`/`alertStories`; mục `Bộ …`
// giữ chuông theo gói và không có hai trường đó.
func Violations(observations []Observation, packID string, total int) []Row {
	out := []Row{}
	for _, g := range groups(observations) {
		stories := g.packStories[packID]
		if len(stories) == 0 {
			continue
		}
		level := "warning"
		if len(g.packErrors[packID]) > 0 {
			level = "error"
		}
		row := Row{Ref: g.first.Ref, RefName: g.first.RefName, Stories: len(stories), Total: total, Level: level, Alert: len(g.packErrors[packID]) >= AlertStories}
		if !IsPackRef(g.first.Ref) {
			row.Alert = len(g.errorStories) >= AlertStories
			row.AlertScope = "all"
			n := len(g.errorStories)
			row.AlertStories = &n
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Stories != out[j].Stories {
			return out[i].Stories > out[j].Stories
		}
		return out[i].Ref < out[j].Ref
	})
	return out
}

// Preamble là lời nhắc trước khi viết truyện mới. Mục dùng chung đang báo động có số
// truyện Bắt sửa tính trên mọi gói khác số trong gói đang xem thì ghi số liên gói
// ("6 truyện ở mọi gói") thay cho "1/4 truyện": con số trong gói giấu mất phạm vi
// thật của mục dùng chung (§J câu 4).
func Preamble(rows []Row, packName string) string {
	refs := []string{}
	for _, row := range rows {
		if !row.Alert {
			continue
		}
		count := fmt.Sprintf("%d/%d truyện", row.Stories, row.Total)
		if row.AlertScope == "all" && row.AlertStories != nil && *row.AlertStories != row.Stories {
			count = fmt.Sprintf("%d truyện ở mọi gói", *row.AlertStories)
		}
		refs = append(refs, fmt.Sprintf("%s (%s, %s)", row.Ref, strings.ToLower(row.RefName), count))
	}
	if len(refs) == 0 {
		return fmt.Sprintf("Chưa có mục Bắt sửa nào lặp ở %d truyện trở lên trong gói %s, nên chưa có lời nhắc.", AlertStories, packName)
	}
	return "Truyện " + packName + " trước đây hay vấp " + strings.Join(refs, ", ") + ". Khi lập bản đồ giữ chân, kiểm kỹ các mục này trước khi giao Người viết."
}
