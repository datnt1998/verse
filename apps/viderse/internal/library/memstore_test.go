package library

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
)

// memStore là Store trong bộ nhớ cho test nghiệp vụ (không SQL): giữ đúng
// ngữ nghĩa quan trọng — CAS revision, tên prompt duy nhất không phân biệt hoa
// thường, tombstone lớp phủ, và kênh dùng template.
type memStore struct {
	mu        sync.Mutex
	templates map[string]domain.Template
	prompts   map[string]domain.ImagePrompt
	overlays  map[string]domain.Overlay
	removed   map[string]bool
	channels  map[string]domain.ChannelRefList
	backdrop  map[string]bool            // kênh nền có thật
	bdVideos  map[string]map[string]bool // channel → video id
	fixed     map[string]bool            // video lẻ có thật
	now       func() time.Time
}

func newMemStore() *memStore {
	return &memStore{
		templates: make(map[string]domain.Template), prompts: make(map[string]domain.ImagePrompt),
		overlays: make(map[string]domain.Overlay), removed: make(map[string]bool),
		channels: make(map[string]domain.ChannelRefList),
		backdrop: make(map[string]bool), bdVideos: make(map[string]map[string]bool),
		fixed: make(map[string]bool), now: func() time.Time { return time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC) },
	}
}

// ── RefChecker ─────────────────────────────────────────────────────────────

func (m *memStore) PromptExists(_ context.Context, id string, kind domain.PromptKind) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.prompts[id]
	return ok && p.Kind == kind, nil
}

func (m *memStore) OverlayExists(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.overlays[id]
	return ok && !m.removed[id], nil
}

func (m *memStore) BackdropChannels(_ context.Context, ids []string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var missing []string
	for _, id := range ids {
		if !m.backdrop[id] {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

func (m *memStore) BackdropVideos(_ context.Context, channelID string, ids []string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var missing []string
	for _, id := range ids {
		if !m.bdVideos[channelID][id] {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

func (m *memStore) BackdropVideoExists(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fixed[id], nil
}

// ── template ───────────────────────────────────────────────────────────────

func (m *memStore) ListTemplates(context.Context) ([]domain.TemplateSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.templates))
	for id := range m.templates {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return m.templates[ids[i]].UpdatedAt.After(m.templates[ids[j]].UpdatedAt) })
	out := make([]domain.TemplateSummary, 0, len(ids))
	for _, id := range ids {
		out = append(out, domain.TemplateSummary{Template: m.templates[id], Channels: append(domain.ChannelRefList{}, m.channels[id]...)})
	}
	return out, nil
}

func (m *memStore) GetTemplate(_ context.Context, id string) (*domain.Template, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tpl, ok := m.templates[id]
	if !ok {
		return nil, nil
	}
	tpl.Project = append(json.RawMessage(nil), tpl.Project...)
	return &tpl, nil
}

func (m *memStore) InsertTemplate(_ context.Context, t domain.Template) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t.Project = append(json.RawMessage(nil), t.Project...)
	m.templates[t.ID] = t
	return nil
}

func (m *memStore) UpdateTemplate(_ context.Context, id string, name *string, recipe domain.Recipe, expectRevision int64, project json.RawMessage) (int64, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Giống bản SQL: kiểm tham chiếu prompt dưới cùng "khoá" (mutex) với ghi.
	for _, ref := range []struct {
		id   string
		kind domain.PromptKind
	}{
		{recipe.Scenes.ScenePromptID, domain.PromptScene},
		{recipe.Scenes.CoverPromptID, domain.PromptCover},
	} {
		if ref.id == "" || IsBuiltinPromptID(ref.id) {
			continue
		}
		p, ok := m.prompts[ref.id]
		if !ok || p.Kind != ref.kind {
			return 0, false, ErrPromptRef
		}
	}
	// Giống bản SQL: tham chiếu video nền kiểm dưới cùng khoá.
	if b := recipe.Backdrop; b != nil {
		for _, link := range b.Channels {
			if link.ChannelID != "" && !m.backdrop[link.ChannelID] {
				return 0, false, ErrBackdropRef
			}
			for _, id := range link.Pinned {
				if !m.bdVideos[link.ChannelID][id] {
					return 0, false, ErrBackdropRef
				}
			}
		}
		for _, id := range b.FixedVideoIDs {
			if id != "" && !m.fixed[id] {
				return 0, false, ErrBackdropRef
			}
		}
	}
	tpl, ok := m.templates[id]
	if !ok || tpl.Revision != expectRevision {
		return 0, false, nil
	}
	tpl.Recipe = recipe
	if project != nil {
		tpl.Project = append(json.RawMessage(nil), project...)
	}
	if name != nil {
		tpl.Name = *name
	}
	tpl.Revision++
	tpl.UpdatedAt = m.now()
	m.templates[id] = tpl
	return tpl.Revision, true, nil
}

func (m *memStore) InitializeTemplateProject(_ context.Context, id string, project json.RawMessage, expectRevision int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tpl, ok := m.templates[id]
	if !ok || tpl.Revision != expectRevision || len(tpl.Project) > 0 {
		return false, nil
	}
	tpl.Project = append(json.RawMessage(nil), project...)
	m.templates[id] = tpl
	return true, nil
}

func (m *memStore) RenameTemplate(_ context.Context, id, name string, expectRevision *int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tpl, ok := m.templates[id]
	if !ok || (expectRevision != nil && tpl.Revision != *expectRevision) {
		return false, nil
	}
	tpl.Name = name
	if len(tpl.Project) > 0 {
		project, err := editor.Parse(tpl.Project)
		if err != nil {
			return false, err
		}
		project["name"] = name
		tpl.Project, err = editor.Marshal(project)
		if err != nil {
			return false, err
		}
	}
	m.templates[id] = tpl
	return true, nil
}

func (m *memStore) NameChannelStandardTemplates(_ context.Context, channelID, plain, name string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ids []string
	for id, tpl := range m.templates {
		users := m.channels[id]
		if tpl.Name != plain || len(users) == 0 {
			continue
		}
		onlyThis := true
		for _, ch := range users {
			if ch.ID != channelID {
				onlyThis = false
			}
		}
		if onlyThis {
			tpl.Name = name
			m.templates[id] = tpl
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (m *memStore) DeleteTemplate(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.channels[id]) > 0 {
		return ErrInUse
	}
	delete(m.templates, id)
	return nil
}

func (m *memStore) TemplateChannels(_ context.Context, templateID string) (domain.ChannelRefList, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append(domain.ChannelRefList{}, m.channels[templateID]...), nil
}

func (m *memStore) TemplatesUsingPrompt(_ context.Context, promptID string) ([]TemplateRef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []TemplateRef
	for _, tpl := range m.templates {
		if tpl.Recipe.Scenes.ScenePromptID == promptID || tpl.Recipe.Scenes.CoverPromptID == promptID {
			out = append(out, TemplateRef{ID: tpl.ID, Name: tpl.Name})
		}
	}
	return out, nil
}

// ── prompt ─────────────────────────────────────────────────────────────────

func (m *memStore) ListPrompts(_ context.Context, kind domain.PromptKind, q string) ([]domain.ImagePrompt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.ImagePrompt, 0, len(m.prompts))
	for _, p := range m.prompts {
		if kind != "" && p.Kind != kind {
			continue
		}
		if q != "" && !strings.Contains(foldText(p.Name), q) && !strings.Contains(foldText(p.Prompt), q) {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (m *memStore) GetPrompt(_ context.Context, id string) (*domain.ImagePrompt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.prompts[id]
	if !ok {
		return nil, nil
	}
	return &p, nil
}

func (m *memStore) NameTaken(_ context.Context, name, exceptID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, p := range m.prompts {
		if id != exceptID && strings.EqualFold(p.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

func (m *memStore) InsertPrompt(_ context.Context, p domain.ImagePrompt) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.prompts {
		if strings.EqualFold(existing.Name, p.Name) {
			return ErrNameTaken
		}
	}
	m.prompts[p.ID] = p
	return nil
}

func (m *memStore) UpdatePrompt(_ context.Context, p domain.ImagePrompt) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.prompts[p.ID]
	if ok && current.Kind != p.Kind {
		for _, tpl := range m.templates {
			if tpl.Recipe.Scenes.ScenePromptID == p.ID || tpl.Recipe.Scenes.CoverPromptID == p.ID {
				return ErrInUse
			}
		}
	}
	for id, existing := range m.prompts {
		if id != p.ID && strings.EqualFold(existing.Name, p.Name) {
			return ErrNameTaken
		}
	}
	m.prompts[p.ID] = p
	return nil
}

func (m *memStore) DeletePrompt(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, tpl := range m.templates {
		if tpl.Recipe.Scenes.ScenePromptID == id || tpl.Recipe.Scenes.CoverPromptID == id {
			return ErrInUse
		}
	}
	delete(m.prompts, id)
	return nil
}

// ── lớp phủ ────────────────────────────────────────────────────────────────

func (m *memStore) ListOverlays(context.Context) ([]domain.Overlay, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.Overlay, 0, len(m.overlays))
	for _, o := range m.overlays {
		if !m.removed[o.ID] {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (m *memStore) OverlayUsage(context.Context) (map[string][]TemplateRef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string][]TemplateRef)
	for _, tpl := range m.templates {
		for _, id := range tpl.Recipe.Overlays {
			out[id] = append(out[id], TemplateRef{ID: tpl.ID, Name: tpl.Name})
		}
	}
	return out, nil
}

func (m *memStore) OverlayByID(_ context.Context, id string) (*domain.Overlay, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.overlays[id]
	if !ok || m.removed[id] {
		return nil, nil
	}
	return &o, nil
}

func (m *memStore) OverlayBySHA(_ context.Context, sha string) (*domain.Overlay, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, o := range m.overlays {
		if o.SHA256 == sha {
			return &o, m.removed[o.ID], nil
		}
	}
	return nil, false, nil
}

func (m *memStore) InsertOverlay(_ context.Context, o domain.Overlay) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.overlays {
		if existing.SHA256 == o.SHA256 {
			return ErrOverlayExists
		}
		if o.Builtin != "" && existing.Builtin == o.Builtin {
			return ErrOverlayExists
		}
	}
	m.overlays[o.ID] = o
	return nil
}

func (m *memStore) RestoreOverlay(_ context.Context, id, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.overlays[id]
	if !ok {
		return ErrNotFound
	}
	o.Name = name
	m.overlays[id] = o
	delete(m.removed, id)
	return nil
}

func (m *memStore) RemoveOverlay(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.overlays[id]; !ok {
		return ErrNotFound
	}
	m.removed[id] = true
	return nil
}

func (m *memStore) ExistingBuiltins(context.Context) (map[string]bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]bool)
	for _, o := range m.overlays {
		if o.Builtin != "" {
			out[o.Builtin] = true
		}
	}
	return out, nil
}
