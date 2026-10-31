package web

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/platform/identity"
	"golang.org/x/text/unicode/norm"
)

type activityCursor struct {
	Size   int64  `json:"s"`
	Before int    `json:"b"`
	Filter string `json:"f"`
}
type activityCounts struct {
	Total   int            `json:"total"`
	ByAgent map[string]int `json:"byAgent"`
	Errors  int            `json:"errors"`
	Mine    int            `json:"mine"`
}
type activityChapter struct {
	Chapter    int      `json:"chapter"`
	Title      string   `json:"title"`
	Score      *float64 `json:"score,omitempty"`
	EventCount int      `json:"eventCount"`
}
type activityHistory struct {
	Events     []activityEvent   `json:"events"`
	NextCursor *string           `json:"nextCursor"`
	Chapters   []activityChapter `json:"chapters"`
	Counts     activityCounts    `json:"counts"`
}

// activityEvent = sự kiện nhật ký + payload thẩm duyệt (D4). Nhúng host.Event nên
// JSON cũ giữ nguyên; dòng Biên tập viên có thêm "review" khi chương đã được chấm.
type activityEvent struct {
	host.Event
	Review *reviewEventAPI `json:"review,omitempty"`
}

func foldActivity(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if r == 'đ' {
			r = 'd'
		}
		b.WriteRune(r)
	}
	return b.String()
}

var activityChapterPattern = regexp.MustCompile(`(?i)(?:chương|chapter|第)\s*(\d+)`)

// parseLogFields accepts the standard slog text format, including quoted JSON.
func parseLogFields(line string) map[string]string {
	out := map[string]string{}
	for len(line) > 0 {
		line = strings.TrimLeft(line, " \t")
		key, rest, ok := strings.Cut(line, "=")
		if !ok || strings.ContainsAny(key, " \t") {
			break
		}
		if strings.HasPrefix(rest, "\"") {
			end := 1
			for end < len(rest) {
				if rest[end] == '\\' {
					end += 2
					continue
				}
				if rest[end] == '"' {
					break
				}
				end++
			}
			if end >= len(rest) {
				break
			}
			v, err := strconv.Unquote(rest[:end+1])
			if err != nil {
				break
			}
			out[key] = v
			line = rest[end+1:]
		} else {
			end := strings.IndexAny(rest, " \t")
			if end < 0 {
				out[key] = rest
				break
			}
			out[key] = rest[:end]
			line = rest[end:]
		}
	}
	return out
}

// activityPathFixer 把事件文本里的绝对路径改写成相对故事目录 / thư viện 的形式（E2EM-4）。
// 旧日志可能写于thư viện 搬家/复制之前：凡以"故事在库内的相对路径"（或其上层项目目录）
// 结尾的外来绝对前缀，先挪到当前位置，再统一相对化。
func activityPathFixer(storyDir, libRoot string) func(string) string {
	var moved []*regexp.Regexp
	var targets []string
	if rel, err := filepath.Rel(libRoot, storyDir); libRoot != "" && err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		parts := strings.Split(filepath.ToSlash(rel), "/")
		for i := len(parts); i >= 1; i-- {
			suffix := strings.Join(parts[:i], "/")
			moved = append(moved, regexp.MustCompile(`~?(?:/[^\s"'/]+)*?/`+regexp.QuoteMeta(suffix)+`(/|[^\p{L}\p{N}._-]|$)`))
			targets = append(targets, filepath.Join(libRoot, filepath.FromSlash(suffix)))
		}
	}
	return func(s string) string {
		if !strings.Contains(s, "/") {
			return s
		}
		for i, re := range moved {
			s = re.ReplaceAllString(s, strings.ReplaceAll(targets[i], "$", "$$")+"${1}")
		}
		s = host.RootRelativeText(s, storyDir, libRoot)
		// Thư mục chính của máy đã ghi log (có thể khác HOME hiện tại) → "~/".
		return activityForeignHome.ReplaceAllString(s, "${1}~/")
	}
}

var activityForeignHome = regexp.MustCompile(`(^|[\s"'(→])/(?:Users|home)/[^/\s"']+/`)

// readActivity 读 tui.log 中的事件行。旧版日志的中文文案经 host.LegacyEvent 转成越南语，
// 路径经 activityPathFixer 相对化。
func readActivity(path string, size int64, storyDir, libRoot string) ([]host.Event, error) {
	fixPaths := activityPathFixer(storyDir, libRoot)
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []host.Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	reader := bufio.NewReader(io.LimitReader(f, size))
	events := []host.Event{}
	indexes := map[string]int{}
	offset := int64(0)
	chapter := 0
	for {
		raw, readErr := reader.ReadString('\n')
		start := offset
		offset += int64(len(raw))
		if readErr != nil {
			if readErr != io.EOF {
				return nil, readErr
			}
			break
		} // Ignore an in-flight partial append.
		fields := parseLogFields(strings.TrimSpace(raw))
		if fields["module"] != "event" {
			continue
		}
		var ev host.Event
		if fields["event_data"] == "" || json.Unmarshal([]byte(fields["event_data"]), &ev) != nil {
			ev.Time, _ = time.Parse(time.RFC3339Nano, fields["time"])
			ev.Summary = fields["summary"]
			if ev.Summary == "" {
				ev.Summary = fields["msg"]
			}
			ev.ID, ev.Agent, ev.Category, ev.Kind = fields["event_id"], fields["agent"], fields["category"], fields["kind"]
			ev.Level = strings.ToLower(fields["level"])
			ev.Failed = fields["failed"] == "true"
			if d, e := time.ParseDuration(fields["duration"]); e == nil {
				ev.Duration = d
			}
			if fields["state"] == "completed" {
				ev.FinishedAt = ev.Time
				ev.Status = "ok"
				if ev.Failed {
					ev.Status = "error"
				}
			}
			if ev.Category == "TOOL" {
				if parts := strings.Fields(ev.Summary); len(parts) > 0 {
					ev.Tool = parts[0]
				}
				ev.Status = "running"
				if fields["state"] == "completed" {
					ev.Status = "ok"
					if ev.Failed {
						ev.Status = "error"
					}
				}
			}
			ev.SessionID = fields["session"]
			ev.Username = fields["username"]
		}
		var visible bool
		ev, visible = normalizeActivityEvent(ev, fixPaths, &chapter)
		if !visible {
			continue
		}
		key := fields["session"] + "/" + ev.SessionID + "/" + ev.ID
		if ev.ID == "" {
			key = fmt.Sprintf("line-%d", start)
		}
		if fields["event_data"] == "" || ev.ID == "" {
			ev.ID = key
		}
		if i, ok := indexes[key]; ok {
			events[i] = ev
		} else {
			indexes[key] = len(events)
			events = append(events, ev)
		}
	}
	events = host.DeduplicateUserEvents(events)
	sort.SliceStable(events, func(i, j int) bool { return events[i].Time.After(events[j].Time) })
	return events, nil
}

// normalizeActivityEvent applies the same legacy text, path privacy and chapter
// context rules to both the desktop log and the durable hosted runtime queue.
func normalizeActivityEvent(ev host.Event, fixPaths func(string) string, chapter *int) (host.Event, bool) {
	if ev.Internal() {
		return ev, false
	}
	if ev.Category == "USER" {
		ev.Summary, ev.Detail = host.LegacyUserText(ev.Summary), host.LegacyUserText(ev.Detail)
	} else {
		ev = host.LegacyEvent(ev)
	}
	ev = host.MapEventText(ev, fixPaths)
	if ev.Chapter == 0 {
		if m := activityChapterPattern.FindStringSubmatch(ev.Summary); len(m) > 1 {
			ev.Chapter, _ = strconv.Atoi(m[1])
		}
	}
	if ev.Chapter > 0 {
		*chapter = ev.Chapter
	} else {
		ev.Chapter = *chapter
	}
	ev.Args, ev.Result = host.EventText(ev.Args, 400), host.EventText(ev.Result, 800)
	ev.DurationMs = ev.Duration.Milliseconds()
	return ev, true
}

// readRuntimeActivity reads the observer's persisted Event payload, rather than
// returning an empty placeholder for hosted stories. Seq pins pagination to a
// stable snapshot while the engine continues appending.
func readRuntimeActivity(queue []domain.RuntimeQueueItem, through int64) ([]host.Event, error) {
	events := []host.Event{}
	indexes := map[string]int{}
	chapter := 0
	fixPaths := activityPathFixer("", "")
	for _, item := range queue {
		if item.Seq > through {
			continue
		}
		var ev host.Event
		if item.Payload != nil {
			data, err := json.Marshal(item.Payload)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(data, &ev); err != nil {
				return nil, err
			}
		}
		if ev.Time.IsZero() {
			ev.Time = item.Time
		}
		if ev.Summary == "" {
			ev.Summary = item.Summary
		}
		if ev.Agent == "" {
			ev.Agent = item.Agent
		}
		if ev.Category == "" {
			ev.Category = item.Category
		}
		var visible bool
		ev, visible = normalizeActivityEvent(ev, fixPaths, &chapter)
		if !visible {
			continue
		}
		if ev.ID == "" {
			ev.ID = fmt.Sprintf("queue-%d", item.Seq)
		}
		key := ev.SessionID + "/" + ev.ID
		if i, ok := indexes[key]; ok {
			events[i] = ev
		} else {
			indexes[key] = len(events)
			events = append(events, ev)
		}
	}
	events = host.DeduplicateUserEvents(events)
	sort.SliceStable(events, func(i, j int) bool { return events[i].Time.After(events[j].Time) })
	return events, nil
}

func (g *Gateway) serveActivityHistory(w http.ResponseWriter, r *http.Request, user identity.User, sr *host.StoryReader) {
	q := r.URL.Query()
	limit := 100
	if v := q.Get("limit"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 500 {
			jsonError(w, 400, "Số sự kiện phải từ 1 đến 500")
			return
		}
		limit = n
	}
	chapter := 0
	if v := q.Get("chapter"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 {
			jsonError(w, 400, "Chương không hợp lệ")
			return
		}
		chapter = n
	}
	if q.Get("level") != "" && q.Get("level") != "error" {
		jsonError(w, 400, "Mức nhật ký không hợp lệ")
		return
	}
	path, storyDir := "", ""
	var queue []domain.RuntimeQueueItem
	size := int64(0)
	if sr.EngineDir() == "" {
		var err error
		queue, err = sr.Store().Runtime.LoadQueue()
		if err != nil {
			jsonError(w, 500, "Không đọc được nhật ký")
			return
		}
		for _, item := range queue {
			size = max(size, item.Seq)
		}
	} else {
		path = filepath.Join(sr.EngineDir(), "logs", "tui.log")
		storyDir = filepath.Dir(filepath.Dir(sr.EngineDir()))
		stat, err := os.Stat(path)
		if err == nil {
			size = stat.Size()
		} else if !errors.Is(err, os.ErrNotExist) {
			jsonError(w, 500, "Không đọc được nhật ký")
			return
		}
	}
	filter := fmt.Sprintf("%s|%s|%s|%s|%s|%s", q.Get("chapter"), q.Get("agent"), q.Get("level"), q.Get("mine"), q.Get("q"), user.Username)
	cursor := activityCursor{Size: size, Before: -1, Filter: filter}
	if before := q.Get("before"); before != "" {
		raw, e := base64.RawURLEncoding.DecodeString(before)
		if e != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Size < 0 || cursor.Size > size || cursor.Before < 0 || cursor.Filter != filter {
			jsonError(w, 400, "Mốc nhật ký không hợp lệ; hãy tải lại")
			return
		}
	}
	var events []host.Event
	var err error
	if sr.EngineDir() == "" {
		events, err = readRuntimeActivity(queue, cursor.Size)
	} else {
		events, err = readActivity(path, cursor.Size, storyDir, g.RootPath())
	}
	if err != nil {
		jsonError(w, 500, "Không đọc được nhật ký")
		return
	}
	sr.ContextualizeEvents(events)
	result := activityHistory{Events: []activityEvent{}, Chapters: []activityChapter{}, Counts: activityCounts{ByAgent: map[string]int{}}}
	groups := map[int]*activityChapter{}
	metadata, _ := sr.Chapters()
	titles := map[int]string{}
	for _, c := range metadata {
		titles[c.Chapter] = c.Title
	}
	filtered := []activityEvent{}
	needle := foldActivity(q.Get("q"))
	for _, ev := range events {
		if q.Has("chapter") && ev.Chapter != chapter {
			continue
		}
		if needle != "" && !strings.Contains(foldActivity(ev.Summary+" "+ev.Tool+" "+ev.Args+" "+ev.Result+" "+ev.Username), needle) {
			continue
		}
		result.Counts.Total++
		result.Counts.ByAgent[activityAgent(ev.Agent)]++
		isError := ev.Failed || ev.Level == "error" || ev.Status == "error" || len(ev.Retries) > 0
		if isError {
			result.Counts.Errors++
		}
		if ev.Username == user.Username && ev.Username != "" {
			result.Counts.Mine++
		}
		if q.Get("agent") != "" && activityAgent(ev.Agent) != activityAgent(q.Get("agent")) {
			continue
		}
		if q.Get("level") == "error" && !isError {
			continue
		}
		if q.Get("mine") == "1" && ev.Username != user.Username {
			continue
		}
		filtered = append(filtered, activityEvent{Event: ev})
		group := groups[ev.Chapter]
		if group == nil {
			group = &activityChapter{Chapter: ev.Chapter, Title: titles[ev.Chapter]}
			if ev.Chapter == 0 {
				group.Title = "Hệ thống"
			}
			if rev, e := sr.Reviews(ev.Chapter); e == nil && rev.Chapter != nil && len(rev.Chapter.Dimensions) > 0 {
				sum := 0
				for _, d := range rev.Chapter.Dimensions {
					sum += d.Score
				}
				score := float64(sum) / float64(len(rev.Chapter.Dimensions))
				group.Score = &score
			}
			groups[ev.Chapter] = group
		}
		group.EventCount++
	}
	for _, group := range groups {
		result.Chapters = append(result.Chapters, *group)
	}
	sort.Slice(result.Chapters, func(i, j int) bool { return result.Chapters[i].Chapter > result.Chapters[j].Chapter })
	start := cursor.Before
	if start < 0 {
		start = 0
	}
	if start > len(filtered) {
		jsonError(w, 400, "Mốc nhật ký không hợp lệ; hãy tải lại")
		return
	}
	end := min(start+limit, len(filtered))
	// Chỉ trang trả về mới cần payload thẩm duyệt (tối đa limit dòng).
	result.Events = filtered[start:end]
	attachActivityReviews(newReviewReader(storyDir, sr), result.Events)
	if end < len(filtered) {
		cursor.Before = end
		raw, _ := json.Marshal(cursor)
		next := base64.RawURLEncoding.EncodeToString(raw)
		result.NextCursor = &next
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, result)
}

func activityAgent(agent string) string {
	if agent == "architect_short" || agent == "architect_long" {
		return "architect"
	}
	return agent
}
