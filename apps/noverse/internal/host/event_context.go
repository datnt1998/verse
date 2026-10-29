package host

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/datnt1998/noverse/internal/domain"
)

// These are the task sentences emitted by flow/router.go, not arbitrary prose.
var eventWorkChapter = regexp.MustCompile(`(?i)(?:Viết(?: lại)?|Gọt giũa|Thẩm duyệt|Đọc) chương\s+(\d+)`)
var eventArc = regexp.MustCompile(`(?i)cung\s+(\d+)`)
var eventVolume = regexp.MustCompile(`(?i)tập\s+(\d+)`)
var eventSpan = regexp.MustCompile(`(?i)chương\s+(\d+)\s*[-–]\s*(\d+)`)

func eventNumber(re *regexp.Regexp, text string) int {
	if match := re.FindStringSubmatch(text); len(match) > 1 {
		n, _ := strconv.Atoi(match[1])
		return n
	}
	return 0
}

// ContextualizeEvent restores the target from event details before using the
// engine cursor. Old dispatch rows can contain the cursor of the *next* chapter.
// Arc bounds come only from a recorded task range or the persisted outline.
func ContextualizeEvent(ev Event, volumes []domain.VolumeOutline) Event {
	if strings.EqualFold(ev.Category, "USER") {
		return ev
	}
	var args struct {
		Chapter int    `json:"chapter"`
		Scope   string `json:"scope"`
		Volume  int    `json:"volume"`
		Arc     int    `json:"arc"`
	}
	// Results remain valid JSON when verbose arguments were truncated in old logs.
	if ev.Result != "" {
		_ = json.Unmarshal([]byte(ev.Result), &args)
	}
	if args.Scope != "" {
		ev.Scope = args.Scope
	}
	if args.Chapter > 0 {
		ev.Chapter = args.Chapter
	}
	args.Chapter, args.Scope, args.Volume, args.Arc = 0, "", 0, 0
	if ev.Tool == "novel_context" && strings.TrimSpace(ev.Args) == "" {
		ev.Chapter = 0
	}
	if ev.Args != "" && json.Unmarshal([]byte(ev.Args), &args) == nil {
		if args.Chapter > 0 {
			ev.Chapter = args.Chapter
		} else if ev.Tool == "novel_context" {
			ev.Chapter = 0
		}
		if args.Scope != "" {
			ev.Scope = args.Scope
		}
		if args.Volume > 0 {
			ev.Volume = args.Volume
		}
		if args.Arc > 0 {
			ev.Arc = args.Arc
		}
	}
	if strings.EqualFold(ev.Category, "DISPATCH") {
		if chapter := eventNumber(eventWorkChapter, ev.Summary); chapter > 0 {
			ev.Chapter = chapter
		}
		summary := strings.ToLower(ev.Summary)
		if strings.Contains(summary, "thẩm duyệt") && strings.Contains(summary, "cung") {
			ev.Scope = "arc"
			ev.Arc = eventNumber(eventArc, ev.Summary)
			ev.Volume = eventNumber(eventVolume, ev.Summary)
			if span := eventSpan.FindStringSubmatch(ev.Summary); len(span) == 3 {
				ev.ChapterFrom, _ = strconv.Atoi(span[1])
				ev.ChapterTo, _ = strconv.Atoi(span[2])
				ev.Chapter = ev.ChapterTo
			}
		}
	}
	if ev.Scope != "arc" || ev.ChapterFrom > 0 && ev.ChapterTo > 0 {
		return ev
	}
	chapter := 1
	for _, volume := range volumes {
		for _, arc := range volume.Arcs {
			from, to := chapter, chapter+len(arc.Chapters)-1
			chapter = to + 1
			if len(arc.Chapters) == 0 {
				continue
			}
			matches := ev.Chapter >= from && ev.Chapter <= to
			if ev.Volume > 0 && ev.Arc > 0 {
				matches = ev.Volume == volume.Index && ev.Arc == arc.Index
			}
			if matches {
				ev.Volume, ev.Arc, ev.ChapterFrom, ev.ChapterTo = volume.Index, arc.Index, from, to
				return ev
			}
		}
	}
	return ev
}

// ContextualizeEvents enriches disk history with the same fields as live events.
// Load the outline once for the whole response, never infer an eight-chapter arc.
func (r *StoryReader) ContextualizeEvents(events []Event) {
	var volumes []domain.VolumeOutline
	outlineLoaded := false
	var dispatches []Event
	for i := range events {
		events[i] = ContextualizeEvent(events[i], nil)
		if events[i].Category == "DISPATCH" && events[i].Scope == "arc" && events[i].ChapterFrom > 0 {
			dispatches = append(dispatches, events[i])
		}
	}
	for i, ev := range events {
		if ev.Scope == "arc" && ev.ChapterFrom == 0 {
			for _, dispatch := range dispatches {
				if ev.Agent == dispatch.Agent && !ev.Time.Before(dispatch.Time) && (dispatch.FinishedAt.IsZero() || !ev.Time.After(dispatch.FinishedAt)) && ev.Chapter >= dispatch.ChapterFrom && ev.Chapter <= dispatch.ChapterTo {
					ev = eventArcContext(ev, dispatch)
					break
				}
			}
			if ev.ChapterFrom == 0 {
				if !outlineLoaded {
					volumes, _ = r.store.Outline.LoadLayeredOutline()
					outlineLoaded = true
				}
				ev = ContextualizeEvent(ev, volumes)
			}
		}
		events[i] = ev
	}
}

func eventArcContext(ev, dispatch Event) Event {
	if ev.Scope == "arc" && ev.ChapterFrom == 0 && dispatch.Scope == "arc" {
		ev.Volume, ev.Arc, ev.ChapterFrom, ev.ChapterTo = dispatch.Volume, dispatch.Arc, dispatch.ChapterFrom, dispatch.ChapterTo
	}
	return ev
}

func (o *observer) contextualizeEvent(ev Event) Event {
	if call := o.toolStarts[ev.Agent]; call != nil && ev.Category == "TOOL" && ev.ID == call.id {
		ev.Scope, ev.Volume, ev.Arc, ev.Chapter = call.scope, call.volume, call.arc, call.chapter
	}
	if call := o.dispatchStarts[ev.Agent]; call != nil && ev.Scope == "arc" {
		ev = eventArcContext(ev, ContextualizeEvent(Event{Category: "DISPATCH", Summary: call.summary}, nil))
	}
	return ev
}
