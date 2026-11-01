package channels

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// fakeStore là Store trong bộ nhớ, giữ đúng các bất biến Service dựa vào: thứ
// tự tên, tham chiếu = link đầu theo linked_at, position liền mạch, lease có
// hạn, publish fence theo token + tên đọc lúc bắt đầu.
type fakeStore struct {
	mu        sync.Mutex
	now       func() time.Time
	channels  map[string]*fakeChannel
	rivals    map[string]fakeRival
	templates map[string]string
	samples   map[string]LanguageSample
	links     int

	// lỗi tiêm để kiểm đường bù trừ
	createErr error
	// nextExpiry giảm trả của NextIdentityLeaseExpiry cho test timer.
	nextExpiry *time.Time
	// acquireGate, khi khác nil, chặn AcquireIdentityLease cho tới khi đóng —
	// test gate prepare vs Close.
	acquireGate  chan struct{}
	acquireEnter chan struct{}
}

type fakeChannel struct {
	c          domain.Channel
	sources    []domain.ChannelSource
	templates  []string
	leaseToken string
	leaseUntil time.Time
}

type fakeRival struct {
	title, description, avatar string
	handle                     string
}

func newFakeStore(now func() time.Time) *fakeStore {
	return &fakeStore{
		now:       now,
		channels:  map[string]*fakeChannel{},
		rivals:    map[string]fakeRival{},
		templates: map[string]string{},
		samples:   map[string]LanguageSample{},
	}
}

func (f *fakeStore) addRival(id, title string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rivals[id] = fakeRival{title: title, handle: "@" + id}
}

func (f *fakeStore) addTemplate(id, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.templates[id] = name
}

func (f *fakeStore) view(fc *fakeChannel) ChannelView {
	out := ChannelView{Channel: fc.c}
	out.Sources = append([]domain.ChannelSource(nil), fc.sources...)
	for i := range out.Sources {
		out.Sources[i].Reference = i == 0
		if out.Sources[i].Handle == nil {
			out.Sources[i].Handle = nil
		}
	}
	out.Templates = make([]domain.ChannelTemplate, 0, len(fc.templates))
	for i, id := range fc.templates {
		out.Templates = append(out.Templates, domain.ChannelTemplate{ID: id, Name: f.templates[id], Position: i})
	}
	return out
}

func (f *fakeStore) ListChannels(context.Context) ([]ChannelView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	list := make([]ChannelView, 0, len(f.channels))
	for _, fc := range f.channels {
		list = append(list, f.view(fc))
	}
	sort.SliceStable(list, func(i, j int) bool {
		ei, ej := list[i].Name == "", list[j].Name == ""
		if ei != ej {
			return !ei
		}
		li, lj := strings.ToLower(list[i].Name), strings.ToLower(list[j].Name)
		if li != lj {
			return li < lj
		}
		return list[i].ID < list[j].ID
	})
	return list, nil
}

func (f *fakeStore) GetChannel(_ context.Context, id string) (ChannelView, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[id]
	if !ok {
		return ChannelView{}, false, nil
	}
	return f.view(fc), true, nil
}

func (f *fakeStore) ChannelExists(_ context.Context, id string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.channels[id]
	return ok, nil
}

func (f *fakeStore) RivalExists(_ context.Context, rivalID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.rivals[rivalID]
	return ok, nil
}

func (f *fakeStore) TemplateExists(_ context.Context, templateID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.templates[templateID]
	return ok, nil
}

func (f *fakeStore) CreateChannel(_ context.Context, in CreateInput) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return "", f.createErr
	}
	for _, rivalID := range in.CompetitorIDs {
		if _, ok := f.rivals[rivalID]; !ok {
			return "", &Error{Code: CodeNotFound, Message: "Đối thủ " + rivalID + " chưa được theo dõi."}
		}
	}
	now := f.now()
	fc := &fakeChannel{c: domain.Channel{
		ID: in.ID, Name: in.Name, YoutubeURL: in.YoutubeURL, Language: in.Language,
		AutoPick: in.AutoPick, PacePerDay: in.PacePerDay, AlwaysTags: []string{},
		IdentityState: domain.IdentityIdle, CreatedAt: now, UpdatedAt: now,
	}}
	for _, rivalID := range in.CompetitorIDs {
		f.links++
		fc.sources = append(fc.sources, domain.ChannelSource{
			RivalID: rivalID, Title: f.rivals[rivalID].title,
			LinkedAt: now.Add(time.Duration(f.links) * time.Millisecond),
		})
	}
	if in.TemplateID != "" {
		fc.templates = append(fc.templates, in.TemplateID)
	}
	f.channels[in.ID] = fc
	return in.ID, nil
}

func (f *fakeStore) PatchChannel(_ context.Context, id string, p Patch) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[id]
	if !ok {
		return false, nil
	}
	if p.Name.Set {
		fc.c.Name = p.Name.Value
	}
	if p.YoutubeURL.Set {
		fc.c.YoutubeURL = p.YoutubeURL.Value
	}
	if p.Language.Set {
		fc.c.Language = p.Language.Value
	}
	if p.StyleNote.Set {
		fc.c.StyleNote = p.StyleNote.Value
	}
	if p.DescriptionFooter.Set {
		fc.c.DescriptionFooter = p.DescriptionFooter.Value
	}
	if p.AlwaysTags.Set {
		fc.c.AlwaysTags = p.AlwaysTags.Value
	}
	if p.AutoPick.Set {
		fc.c.AutoPick = p.AutoPick.Value
	}
	if p.PacePerDay.Set {
		fc.c.PacePerDay = p.PacePerDay.Value
	}
	if p.Brief.Set {
		fc.c.Branding.Brief = p.Brief.Value
	}
	fc.c.UpdatedAt = f.now()
	return true, nil
}

func (f *fakeStore) DeleteChannel(_ context.Context, id string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.channels[id]; !ok {
		return false, nil
	}
	delete(f.channels, id)
	return true, nil
}

func (f *fakeStore) LinkSource(_ context.Context, id, rivalID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[id]
	if !ok {
		return false, nil
	}
	for _, s := range fc.sources {
		if s.RivalID == rivalID {
			return false, nil
		}
	}
	f.links++
	fc.sources = append(fc.sources, domain.ChannelSource{
		RivalID: rivalID, Title: f.rivals[rivalID].title,
		LinkedAt: f.now().Add(time.Duration(f.links) * time.Millisecond),
	})
	return true, nil
}

func (f *fakeStore) UnlinkSource(_ context.Context, id, rivalID string) (bool, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[id]
	if !ok {
		return false, false, nil
	}
	idx := -1
	for i, s := range fc.sources {
		if s.RivalID == rivalID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false, false, nil
	}
	fc.sources = append(fc.sources[:idx], fc.sources[idx+1:]...)
	if len(fc.sources) == 0 {
		fc.c.AutoPick = false
		return true, true, nil
	}
	return true, false, nil
}

func (f *fakeStore) SetSourceMinDuration(_ context.Context, id, rivalID string, minDurationSec *int) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[id]
	if !ok {
		return false, nil
	}
	for i := range fc.sources {
		if fc.sources[i].RivalID == rivalID {
			fc.sources[i].MinDurationSec = minDurationSec
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeStore) UpdateTemplates(_ context.Context, id string, apply func([]string) []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[id]
	if !ok {
		return &Error{Code: CodeNotFound, Message: "Kênh không tồn tại."}
	}
	fc.templates = append([]string(nil), apply(append([]string(nil), fc.templates...))...)
	return nil
}

func (f *fakeStore) LoadIdentityRun(_ context.Context, id string) (IdentityRun, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[id]
	if !ok {
		return IdentityRun{}, false, nil
	}
	return IdentityRun{
		ID: fc.c.ID, Name: fc.c.Name, StyleNote: fc.c.StyleNote, Branding: fc.c.Branding,
		AvatarPath: fc.c.AvatarPath, BannerPath: fc.c.BannerPath,
	}, true, nil
}

func (f *fakeStore) AcquireIdentityLease(_ context.Context, in LeaseInput) (bool, error) {
	if f.acquireEnter != nil {
		select {
		case f.acquireEnter <- struct{}{}:
		default:
		}
	}
	if f.acquireGate != nil {
		<-f.acquireGate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[in.ChannelID]
	if !ok {
		return false, nil
	}
	if fc.leaseToken != "" && fc.leaseUntil.After(f.now()) {
		return false, nil
	}
	fc.leaseToken = in.Token
	fc.leaseUntil = in.Until
	fc.c.IdentityState = domain.IdentityRunning
	fc.c.IdentityPart = &in.Part
	fc.c.IdentityTaskID = in.TaskID
	fc.c.IdentityErrorCode = nil
	fc.c.IdentityError = nil
	fc.c.IdentityFailedStep = nil
	fc.c.IdentityRetryAt = nil
	started := f.now()
	fc.c.IdentityStartedAt = &started
	return true, nil
}

func (f *fakeStore) HeartbeatIdentityLease(_ context.Context, id, token string, until time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[id]
	if !ok || fc.leaseToken != token || !fc.leaseUntil.After(f.now()) {
		return false, nil
	}
	fc.leaseUntil = until
	return true, nil
}

func (f *fakeStore) IdentityLeaseLive(_ context.Context, id string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[id]
	if !ok {
		return false, nil
	}
	return fc.leaseToken != "" && fc.leaseUntil.After(f.now()), nil
}

func (f *fakeStore) PublishIdentity(_ context.Context, in PublishInput) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[in.ChannelID]
	if !ok {
		return false, nil
	}
	if fc.leaseToken != in.Token || !fc.leaseUntil.After(f.now()) {
		return false, nil
	}
	if fc.c.Name != in.ExpectedName {
		return false, nil
	}
	fc.c.Name = in.Name
	fc.c.Branding = in.Branding
	if in.AvatarPath != "" {
		v := in.AvatarPath
		fc.c.AvatarPath = &v
	}
	if in.BannerPath != "" {
		v := in.BannerPath
		fc.c.BannerPath = &v
	}
	fc.c.IdentityState = domain.IdentityIdle
	fc.c.IdentityPart = nil
	fc.c.IdentityTaskID = nil
	fc.c.IdentityErrorCode = nil
	fc.c.IdentityError = nil
	fc.c.IdentityFailedStep = nil
	fc.c.IdentityRetryAt = nil
	fc.c.IdentityStartedAt = nil
	fc.leaseToken = ""
	fc.leaseUntil = time.Time{}
	return true, nil
}

func (f *fakeStore) FailIdentity(_ context.Context, in FailInput) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[in.ChannelID]
	if !ok || fc.leaseToken != in.Token {
		return false, nil
	}
	fc.c.IdentityState = domain.IdentityFailed
	code := in.Code
	msg := in.Message
	fc.c.IdentityErrorCode = &code
	fc.c.IdentityError = &msg
	fc.c.IdentityRetryAt = in.RetryAt
	fc.c.IdentityFailedStep = nil
	if in.Step != "" {
		step := in.Step
		fc.c.IdentityFailedStep = &step
	}
	fc.leaseToken = ""
	fc.leaseUntil = time.Time{}
	return true, nil
}

func (f *fakeStore) AttachIdentityTask(_ context.Context, id, token, taskID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[id]
	if !ok || fc.leaseToken != token {
		return false, nil
	}
	fc.c.IdentityTaskID = &taskID
	return true, nil
}

func (f *fakeStore) NextIdentityLeaseExpiry(context.Context) (*time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.nextExpiry, nil
}

func (f *fakeStore) ReleaseIdentityLease(_ context.Context, id, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[id]
	if !ok || fc.leaseToken != token {
		return nil
	}
	fc.leaseToken = ""
	fc.leaseUntil = time.Time{}
	return nil
}

func (f *fakeStore) MarkInterruptedIdentities(context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for id, fc := range f.channels {
		if fc.c.IdentityState == domain.IdentityRunning && (fc.leaseToken == "" || !fc.leaseUntil.After(f.now())) {
			fc.c.IdentityState = domain.IdentityFailed
			code := domain.IdentityErrInterrupted
			msg := "Lần tạo nhận diện trước dừng giữa chừng."
			fc.c.IdentityErrorCode = &code
			fc.c.IdentityError = &msg
			fc.c.IdentityFailedStep = nil
			fc.leaseToken = ""
			fc.leaseUntil = time.Time{}
			out = append(out, id)
		}
	}
	return out, nil
}

func (f *fakeStore) ChannelAssetPaths(_ context.Context, id string) (avatar, banner *string, found bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[id]
	if !ok {
		return nil, nil, false, nil
	}
	return fc.c.AvatarPath, fc.c.BannerPath, true, nil
}

func (f *fakeStore) Competitors(_ context.Context, id string) ([]Competitor, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.channels[id]
	if !ok {
		return nil, nil
	}
	out := make([]Competitor, 0, len(fc.sources))
	for _, s := range fc.sources {
		r := f.rivals[s.RivalID]
		var avatar *string
		if r.avatar != "" {
			v := r.avatar
			avatar = &v
		}
		out = append(out, Competitor{ID: s.RivalID, Title: s.Title, Description: r.description, AvatarURL: avatar})
	}
	return out, nil
}

func (f *fakeStore) ReferenceLanguageText(_ context.Context, id string) (LanguageSample, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Như SQL: văn bản của đối thủ tham chiếu (link đầu tiên).
	fc, ok := f.channels[id]
	if !ok || len(fc.sources) == 0 {
		return LanguageSample{}, nil
	}
	return f.samples[fc.sources[0].RivalID], nil
}
