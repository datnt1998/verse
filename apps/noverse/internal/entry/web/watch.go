package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

const watchInterval = 2 * time.Second

type watchedResource struct {
	paths []string
	event Event
	part  string
}
type fileWatcher struct {
	g        *Gateway
	mu       sync.Mutex
	previous map[string]string
	contents map[string]string
	stop     chan struct{}
	done     chan struct{}
}

func (g *Gateway) startWatcher() {
	g.watchMu.Lock()
	defer g.watchMu.Unlock()
	if g.watcher != nil {
		return
	}
	watcher := &fileWatcher{g: g, previous: map[string]string{}, stop: make(chan struct{}), done: make(chan struct{})}
	g.watcher = watcher
	// Establish each new key's baseline before returning the SSE hello, so an
	// external save immediately after subscription cannot become the baseline.
	g.events().OnWatchChange(watcher.prime)
	watcher.scan(false)
	go func() {
		defer close(watcher.done)
		ticker := time.NewTicker(watchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-watcher.stop:
				return
			case <-ticker.C:
				watcher.scan(true)
			}
		}
	}()
}
func (g *Gateway) stopWatcher() {
	g.watchMu.Lock()
	watcher := g.watcher
	g.watcher = nil
	if watcher != nil {
		close(watcher.stop)
	}
	g.watchMu.Unlock()
	if watcher != nil {
		<-watcher.done
	}
}
func (w *fileWatcher) prime() {
	select {
	case <-w.stop:
		return
	default:
	}
	w.scan(false)
}

// Poll metadata cheaply; hash file bytes only when story metadata changes.
// Directory traversal does not follow symlinks outside the library.
func watchFingerprint(paths []string, content bool) string {
	h := sha256.New()
	for _, root := range paths {
		_, _ = fmt.Fprintln(h, root)
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				_, _ = fmt.Fprintf(h, "%s:missing\n", path)
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if !d.IsDir() && strings.HasPrefix(d.Name(), ".") {
				return nil
			}
			if content {
				if d.IsDir() {
					return nil
				}
				_, _ = fmt.Fprintln(h, path)
				file, err := os.Open(path)
				if err != nil {
					_, _ = fmt.Fprintln(h, err)
					return nil
				}
				_, _ = io.Copy(h, file)
				_ = file.Close()
				return nil
			}
			if info, err := d.Info(); err == nil {
				_, _ = fmt.Fprintf(h, "%s:%d:%d\n", path, info.ModTime().UnixNano(), info.Size())
			}
			return nil
		})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (g *Gateway) watchedResources() map[string]watchedResource {
	resources := map[string]watchedResource{}
	addConfig := func(scope, p, s, config, rules string) {
		key := "config:" + scope + ":" + p + "/" + s
		resources[key] = watchedResource{paths: []string{config}, event: Event{Type: "config.changed", Project: p, Story: s, Data: map[string]any{"scope": scope, "project": p, "story": s}}}
		resources["rules:"+key] = watchedResource{paths: []string{rules}, event: Event{Type: "config.changed", Project: p, Story: s, Data: map[string]any{"section": "rules", "scope": scope, "project": p, "story": s}}}
	}
	addConfig("global", "", "", bootstrap.DefaultConfigPath(), rulesHomeDir())
	if path := g.libraryConfigPath(); path != "" {
		resources["library"] = watchedResource{paths: []string{path}, event: Event{Type: "library.changed", Data: map[string]any{}}}
	}
	ctx := context.Background()
	lib, err := g.lib(ctx, nil)
	if err != nil {
		return resources
	}
	stories := map[string]library.Story{}
	for key := range g.events().Watched() {
		p, s, storyKey := strings.Cut(key, "/")
		if p == "." || p == ".." || s == "." || s == ".." {
			continue
		}
		ref, ok := lib.ResolveProject(ctx, nil, p)
		if !ok || ref.Dir == "" {
			continue
		}
		pdir := ref.Dir
		resources["project:"+p] = watchedResource{paths: []string{filepath.Join(pdir, "project.json")}, event: Event{Type: "project.changed", Project: p, Data: map[string]any{"project": p}}}
		addConfig("project", p, "", configFileIn(pdir), rulesDirIn(pdir))
		if storyKey {
			if st, ok := lib.ResolveStory(ctx, nil, p, s); ok {
				stories[st.ID] = st
			}
			continue
		}
		if dirs, err := lib.Stories(ctx, nil, p); err == nil {
			for _, dir := range dirs {
				if st, ok := lib.ResolveStory(ctx, nil, p, dir.Name); ok {
					stories[st.ID] = st
				}
			}
		}
	}
	for _, st := range stories {
		if st.Dir == "" {
			continue
		}
		engine := host.NewStoryReader(st.Dir).EngineDir()
		addConfig("story", st.Project, st.Story, configFileIn(st.Dir), rulesDirIn(st.Dir))
		groups := map[string][]string{
			"chapters": {filepath.Join(engine, "chapters"), filepath.Join(engine, "drafts")},
			"outline":  {filepath.Join(engine, "outline.json"), filepath.Join(engine, "outline.md"), filepath.Join(engine, "layered_outline.json"), filepath.Join(engine, "layered_outline.md")},
			"reviews":  {filepath.Join(engine, "reviews")},
			"rules":    {filepath.Join(engine, "meta", "user_rules.json"), filepath.Join(engine, "meta", "user_rules_runtime.json"), filepath.Join(engine, "meta", "rule_violations.jsonl")},
			"snapshot": {filepath.Join(engine, "meta", "progress.json"), filepath.Join(engine, "meta", "book.json"), filepath.Join(engine, "characters.json"), filepath.Join(engine, "world_rules.json")},
			"meta":     {filepath.Join(st.Dir, "story.json")},
			"exports":  {filepath.Join(st.Dir, lib.Config().Dirs.Exports)},
			"audio":    {filepath.Join(st.Dir, lib.Config().Dirs.Audio), filepath.Join(st.ProjectDir, ".ainovel", "cach-doc.json"), filepath.Join(st.ProjectDir, lib.Config().Dirs.Assets, "giong")},
			"activity": {filepath.Join(engine, "logs", "tui.log")},
			// Não truyện (GAP-38/39): xưng hô & quan hệ, rồi dữ kiện dựng đồ thị.
			// Sửa ngoài app (TUI, sửa tay) phải phát đúng part để web làm mới.
			"address": {
				filepath.Join(engine, "address_terms.json"),
				filepath.Join(engine, "relationship_log.jsonl"),
				filepath.Join(engine, "relationship_state.json"),
			},
			"brain": {
				filepath.Join(engine, "characters.json"),
				filepath.Join(engine, "foreshadow_ledger.json"),
				filepath.Join(engine, "timeline.json"),
				filepath.Join(engine, "timeline.jsonl"),
				filepath.Join(engine, "world_rules.json"),
				filepath.Join(engine, "meta", "cast_ledger.json"),
			},
		}
		for part, paths := range groups {
			resources["story:"+st.ID+":"+part] = watchedResource{paths: paths, event: Event{Type: "story.changed", Project: st.Project, Story: st.Story}, part: part}
		}
	}
	return resources
}

// One polling pass coalesces all edited files into one event per story (the
// debounce window is the polling interval); unchanged files never emit again.
func (w *fileWatcher) scan(publish bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	resources := w.g.watchedResources()
	next := make(map[string]string, len(resources))
	storyParts := map[string]map[string]bool{}
	var bookChanged []string
	if w.contents == nil {
		w.contents = map[string]string{}
	}
	for key, res := range resources {
		fingerprint := watchFingerprint(res.paths, false)
		next[key] = fingerprint
		old, exists := w.previous[key]
		if res.part == "chapters" || res.part == "outline" || res.part == "reviews" {
			priorContent, tracked := w.contents[key]
			if !tracked || (publish && old != fingerprint) {
				content := watchFingerprint(res.paths, true)
				w.contents[key] = content
				if tracked && content == priorContent {
					continue
				}
			}
		}
		if !exists || old == fingerprint || !publish {
			continue
		}
		if res.event.Type == "story.changed" {
			id := res.event.Project + "/" + res.event.Story
			if storyParts[id] == nil {
				storyParts[id] = map[string]bool{}
			}
			storyParts[id][res.part] = true
			storyParts[id]["snapshot"] = true
			// snapshot nhóm chứa meta/book.json: chỉ đổi thật mới kiểm tra đồng bộ tên.
			if res.part == "snapshot" {
				bookChanged = append(bookChanged, id)
			}
		} else {
			w.g.events().Publish(res.event)
		}
	}
	if !publish {
		for key, old := range w.previous {
			if _, ok := next[key]; ok {
				next[key] = old
			}
		}
	}
	w.previous = next
	if !publish {
		return
	}
	// meta/book.json đổi → tên sách vừa được engine đặt/cập nhật: đồng bộ story.json
	// (chỉ khi người dùng chưa đổi tên tay; thư mục giữ nguyên để liên kết không hỏng).
	if lib, err := w.g.lib(context.Background(), nil); err == nil {
		for _, id := range bookChanged {
			p, s, _ := strings.Cut(id, "/")
			if st, ok := lib.ResolveStory(context.Background(), nil, p, s); ok {
				w.g.syncStoryTitle(context.Background(), lib, st, w.g.engineBookTitle(st, nil), nil)
			}
		}
	}
	for id, parts := range storyParts {
		p, s, _ := strings.Cut(id, "/")
		list := make([]string, 0, len(parts))
		for part := range parts {
			list = append(list, part)
		}
		sort.Strings(list)
		w.g.storyChanged(p, s, list...)
	}
}

func (g *Gateway) serveEventsWatch(w http.ResponseWriter, r *http.Request, user webauth.User) {
	var req struct {
		Keys       []string `json:"keys"`
		Subscriber string   `json:"subscriber,omitempty"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	hub := g.events()
	hub.mu.Lock()
	ids := make([]string, 0)
	for id, sub := range hub.subs {
		if sub.user.Username == user.Username && (req.Subscriber == "" || req.Subscriber == id) {
			ids = append(ids, id)
		}
	}
	hub.mu.Unlock()
	if req.Subscriber != "" && len(ids) == 0 {
		jsonError(w, 404, "Không tìm thấy kết nối sự kiện")
		return
	}
	for _, id := range ids {
		hub.SetWatch(id, user.Username, req.Keys)
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
