package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/lessons"
	"github.com/datnt1998/verse/apps/noverse/internal/naolint"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

type lessonPackAPI struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Stories int    `json:"stories"`
}
type lessonStatsAPI struct {
	Projects int `json:"projects"`
	Stories  int `json:"stories"`
	Rounds   int `json:"rounds"`
}
type lessonData struct {
	observations    []lessons.Observation
	packs           []lessonPackAPI
	stats           lessonStatsAPI
	names, passages []string
}

// Lesson text uses only rule headings from the loaded pack. Legacy unreferenced
// prose and runtime user rules have no structured reusable pattern and are not
// promoted into a system rule.
func (g *Gateway) lessonData(user webauth.User) (lessonData, error) {
	data := lessonData{packs: []lessonPackAPI{}}
	packIndex := map[string]int{}
	for _, p := range assets.ListPacks(assets.DefaultLoadOptions("")) {
		packIndex[p.Name] = len(data.packs)
		data.packs = append(data.packs, lessonPackAPI{ID: p.Name, Name: p.Ten})
	}
	if g.RootPath() == "" && g.libRepo == nil {
		return data, nil
	}
	ctx := context.Background()
	actor := actorOf(user)
	lib, err := g.lib(ctx, actor)
	if err != nil {
		return data, err
	}
	scan, err := lib.Scan(ctx, actor)
	if err != nil {
		return data, err
	}
	for _, project := range scan.Projects {
		stories, err := lib.Stories(ctx, actor, project.ID)
		if err != nil {
			return data, err
		}
		visible := false
		for _, sd := range stories {
			id := project.ID + "/" + sd.Name
			if !user.CanRead(id) {
				continue
			}
			st, ok := lib.ResolveStory(ctx, actor, sd.Project, sd.Name)
			if !ok {
				continue
			}
			visible = true
			data.stats.Stories++
			sr, err := g.openStoryReader(user, st)
			if err != nil {
				continue
			}
			reader := newReviewReader(sd.Dir, sr)
			title := "Truyện chưa đặt tên"
			if f, err := lib.LoadStoryFile(ctx, actor, st); err != nil {
				return data, err
			} else if f != nil && f.Title != "" {
				title = f.Title
			}
			data.names = append(data.names, title)
			chars, cast, err := sr.Characters()
			if err != nil {
				return data, err
			}
			for _, c := range chars {
				data.names = append(data.names, c.Name)
				data.names = append(data.names, c.Aliases...)
			}
			for _, c := range cast {
				data.names = append(data.names, c.Name)
				data.names = append(data.names, c.Aliases...)
			}
			if reader.pack == nil {
				continue
			}
			idx, ok := packIndex[reader.pack.ID]
			if !ok {
				idx = len(data.packs)
				packIndex[reader.pack.ID] = idx
				data.packs = append(data.packs, lessonPackAPI{ID: reader.pack.ID, Name: reader.pack.Ten})
			}
			data.packs[idx].Stories++
			cards, err := reader.cards()
			if err != nil {
				return data, err
			}
			for _, card := range cards {
				dto := card.View
				data.stats.Rounds += 1 + len(dto.Previous)
				at, _ := time.Parse(time.RFC3339, dto.At)
				if at.IsZero() {
					if info, err := os.Stat(filepath.Join(sr.EngineDir(), "reviews", card.FileName())); err == nil {
						at = info.ModTime().UTC()
					}
				}
				add := func(items []reviewItemAPI, round int) {
					for _, item := range items {
						data.passages = append(data.passages, item.Quote)
						refName := reader.titles[item.Ref]
						// Mục xưng hô không nằm trong tệp luật của gói: dùng tên mục cố định, không lấy
						// chiều gọi (có tên nhân vật) để câu bài học không mang tên riêng của truyện.
						if refName == "" && item.Ref == reviewRefAddress {
							refName = naolint.IssueName
						}
						if refName == "" {
							continue
						}
						data.observations = append(data.observations, lessons.Observation{ID: fmt.Sprintf("%s:%d:%s", card.FileName(), round, item.ID), Story: id, StoryTitle: title, PackID: reader.pack.ID, PackName: reader.pack.Ten, Ref: item.Ref, RefName: refName, Level: item.Level, Chapter: dto.Chapter, At: at})
					}
				}
				// Bài học gom cả mục xưng hô: "Xưng hô" là mục luật không thuộc
				// gói nào nên nhóm qua mọi gói (câu 15 §H). Thẻ xưng hô chỉ được
				// dựng khi bài chưa gộp mục này vào issues[] (items() dừng sớm), nên
				// mỗi vi phạm chỉ thành một quan sát.
				for _, previous := range dto.Previous {
					add(previous.Items, previous.Round)
				}
				add(dto.Items, dto.Round)
			}
		}
		if visible {
			data.stats.Projects++
		}
	}
	return data, nil
}

func (g *Gateway) systemLessons(user webauth.User) ([]lessons.Lesson, lessonData, error) {
	data, err := g.lessonData(user)
	if err != nil {
		return nil, data, err
	}
	s, err := lessons.Open()
	if err != nil {
		return nil, data, err
	}
	candidates := lessons.Aggregate(data.observations)
	safe := candidates[:0]
	for _, candidate := range candidates {
		if lessons.ValidateText(candidate.Text, data.names, data.passages) == nil {
			safe = append(safe, candidate)
		}
	}
	summaries := make([]lessons.Summary, 0, len(data.packs))
	for _, pack := range data.packs {
		summaries = append(summaries, lessons.Summary{PackID: pack.ID, PackName: pack.Name, Rows: lessons.Violations(data.observations, pack.ID, pack.Stories)})
	}
	list, err := s.Sync(safe, summaries...)
	return list, data, err
}

func (g *Gateway) pendingLessons(user webauth.User) (int, error) {
	list, _, err := g.systemLessons(user)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, l := range list {
		if l.State == "pending" {
			n++
		}
	}
	return n, nil
}

func (g *Gateway) serveLessons(w http.ResponseWriter, r *http.Request, user webauth.User) bool {
	path := r.URL.Path
	if path != "/api/system/lessons" && !strings.HasPrefix(path, "/api/system/lessons/") && path != "/api/system/violations" {
		return false
	}
	if !isAdmin(user) {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return true
	}
	list, data, err := g.systemLessons(user)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được bài học: "+err.Error())
		return true
	}
	switch {
	case path == "/api/system/lessons" && r.Method == http.MethodGet:
		state := r.URL.Query().Get("state")
		if state == "" {
			state = "pending"
		}
		if state != "pending" && state != "kept" && state != "dropped" {
			jsonError(w, 400, "Mục bài học không hợp lệ")
			return true
		}
		counts := map[string]int{"pending": 0, "kept": 0, "dropped": 0}
		selected := []lessons.Lesson{}
		for _, l := range list {
			counts[l.State]++
			if l.State == state {
				selected = append(selected, l)
			}
		}
		writeJSON(w, 200, lessonsResponse{Lessons: selected, Counts: counts, Stats: data.stats, Packs: data.packs})
	case path == "/api/system/violations" && r.Method == http.MethodGet:
		id := r.URL.Query().Get("pack")
		aliases := map[string]string{"A": "nu-tan-bao-thu", "B": "nguoc-hon-nhan", "C": "nien-dai-van"}
		if alias := aliases[id]; alias != "" {
			id = alias
		}
		var pack *lessonPackAPI
		for i := range data.packs {
			if data.packs[i].ID == id {
				pack = &data.packs[i]
				break
			}
		}
		if pack == nil {
			jsonError(w, 400, "Gói truyện không hợp lệ")
			return true
		}
		rows := lessons.Violations(data.observations, id, pack.Stories)
		writeJSON(w, 200, violationsResponse{Rows: rows, Preamble: lessons.Preamble(rows, pack.Name), PackName: pack.Name, Stories: pack.Stories})
	case strings.HasPrefix(path, "/api/system/lessons/") && r.Method == http.MethodPost:
		id := strings.TrimPrefix(path, "/api/system/lessons/")
		var req struct {
			Action string `json:"action"`
			Text   string `json:"text"`
			Scope  string `json:"scope"`
		}
		if !decodeJSON(w, r, &req) {
			return true
		}
		if req.Action == "edit" {
			if err := lessons.ValidateText(req.Text, data.names, data.passages); err != nil {
				jsonError(w, 400, err.Error())
				return true
			}
		}
		s, err := lessons.Open()
		if err != nil {
			jsonError(w, 500, err.Error())
			return true
		}
		lesson, err := s.Decide(id, req.Action, req.Text, req.Scope)
		if errors.Is(err, lessons.ErrNotFound) {
			jsonError(w, 404, err.Error())
		} else if errors.Is(err, lessons.ErrAction) {
			jsonError(w, 400, err.Error())
		} else if err != nil {
			jsonError(w, 500, "Không lưu được bài học")
		} else {
			writeJSON(w, 200, lesson)
		}
	default:
		jsonError(w, http.StatusMethodNotAllowed, "Không hỗ trợ thao tác này")
	}
	return true
}
