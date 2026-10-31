package metadata

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

type View struct {
	Status            string   `json:"status"`
	Metadata          *Content `json:"metadata"`
	Revision          int      `json:"revision"`
	DescriptionFooter *string  `json:"descriptionFooter"`
	AlwaysTags        []string `json:"alwaysTags"`
}
type Service struct {
	Edits    *jobedit.Service
	Generate func(context.Context, jobedit.Snapshot, []string, string) (Content, error)
	Channel  func(context.Context, string) (*domain.Channel, error)
}
type loaded struct {
	snapshot jobedit.Snapshot
	state    State
	view     View
}

func (s *Service) load(ctx context.Context, id string) (loaded, error) {
	var l loaded
	var err error
	l.snapshot, err = s.Edits.Read(ctx, id, domain.CloneMetadata)
	if err != nil {
		return l, err
	}
	l.view.Status = "pending"
	switch l.snapshot.Step.Status {
	case domain.CloneRunning:
		l.view.Status = "writing"
	case domain.CloneFailed, domain.CloneWaiting:
		l.view.Status = "failed"
	}
	if jobedit.Published(l.snapshot, "metadata/metadata.json") {
		h, e := s.Edits.Acquire(ctx, l.snapshot, "metadata/metadata.json")
		if e != nil {
			return l, e
		}
		raw, e := os.ReadFile(h.Path)
		h.Release()
		if e != nil {
			return l, e
		}
		if e = json.Unmarshal(raw, &l.state.Current); e != nil {
			return l, e
		}
		if l.view.Status == "pending" {
			l.view.Status = "ai"
		}
		if len(l.snapshot.Step.Checkpoint) > 0 {
			var state State
			if json.Unmarshal(l.snapshot.Step.Checkpoint, &state) == nil && state.Revision > 0 {
				l.state = state
			}
		}
		l.view.Metadata = &l.state.Current
	}
	l.view.Revision = l.state.Revision
	l.view.AlwaysTags = []string{}
	if s.Channel != nil {
		ch, e := s.Channel(ctx, l.snapshot.Job.ChannelID)
		if e != nil {
			return l, e
		}
		if ch != nil {
			l.view.DescriptionFooter = ch.DescriptionFooter
			l.view.AlwaysTags = ch.AlwaysTags
			if l.view.AlwaysTags == nil {
				l.view.AlwaysTags = []string{}
			}
		}
	}
	return l, nil
}
func (s *Service) Read(ctx context.Context, id string) (View, error) {
	l, e := s.load(ctx, id)
	return l.view, e
}
func (s *Service) History(ctx context.Context, id string) ([]Version, error) {
	l, e := s.load(ctx, id)
	if e != nil {
		return nil, e
	}
	if l.state.History == nil {
		return []Version{}, nil
	}
	return l.state.History, nil
}
func (s *Service) commit(ctx context.Context, l loaded) (View, error) {
	dir, e := os.MkdirTemp("", "viderse-metadata-")
	if e != nil {
		return View{}, e
	}
	defer os.RemoveAll(dir)
	raw, e := json.Marshal(struct {
		Version int `json:"version"`
		Content
	}{1, l.state.Current})
	if e != nil {
		return View{}, e
	}
	file := filepath.Join(dir, "metadata.json")
	if e = os.WriteFile(file, raw, 0600); e != nil {
		return View{}, e
	}
	if e = s.Edits.Commit(ctx, l.snapshot, l.state, "", false, []media.Artifact{{Path: "metadata/metadata.json", File: file, ContentType: "application/json"}}); e != nil {
		return View{}, e
	}
	l.view.Metadata = &l.state.Current
	l.view.Status = "ai"
	l.view.Revision = l.state.Revision
	return l.view, nil
}
func (s *Service) Patch(ctx context.Context, id string, p Patch) (View, error) {
	l, e := s.load(ctx, id)
	if e != nil {
		return View{}, e
	}
	if l.view.Metadata == nil {
		return View{}, invalid("Nội dung chưa được viết")
	}
	if e = l.state.patch(p); e != nil {
		return View{}, e
	}
	return s.commit(ctx, l)
}
func (s *Service) Rewrite(ctx context.Context, id string, r Rewrite) (View, error) {
	l, e := s.load(ctx, id)
	if e != nil {
		return View{}, e
	}
	if e = l.state.check(r.ExpectedRevision); e != nil {
		return View{}, e
	}
	if e = validParts(r.Parts); e != nil {
		return View{}, e
	}
	if s.Generate == nil {
		return View{}, &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Chưa có model viết nội dung"}
	}
	direction := r.Direction
	if r.Feedback != "" {
		direction = r.Feedback
	}
	fresh, e := s.Generate(ctx, l.snapshot, r.Parts, direction)
	if e != nil {
		return View{}, e
	}
	next := merge(l.state.Current, fresh, r.Parts)
	if e = validate(next); e != nil {
		return View{}, e
	}
	edited := map[string]bool{}
	for k, v := range next.Edited {
		edited[k] = v
	}
	for _, p := range r.Parts {
		delete(edited, p)
	}
	next.Edited = edited
	l.state.save(next, "director", r.Parts)
	return s.commit(ctx, l)
}
func (s *Service) Restore(ctx context.Context, id string, r Restore) (View, error) {
	l, e := s.load(ctx, id)
	if e != nil {
		return View{}, e
	}
	if e = l.state.check(r.ExpectedRevision); e != nil {
		return View{}, e
	}
	for _, v := range l.state.History {
		if v.At == r.At {
			l.state.save(v.Content, "owner", []string{"title", "description", "tags"})
			return s.commit(ctx, l)
		}
	}
	return View{}, invalid("Không tìm thấy bản nội dung")
}
