package backdrop_test

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// memStore là Store trong bộ nhớ cho test service: cùng quy ước sắp xếp và
// keyset với Postgres, không cần database.
type memStore struct {
	mu        sync.Mutex
	channels  map[string]domain.BackdropChannel
	channelAt []string
	videos    map[string]map[string]domain.BackdropChannelVideo
	fixed     map[string]domain.BackdropVideo
	fixedAt   []string
	templates []backdrop.TemplateRecipeRow
	requests  map[string]backdrop.PlatformRequestRecord
	// progressWrites đếm số lần ghi tiến độ tải để kiểm throttle.
	progressWrites int
	updateCalls    int

	// Chủ sở hữu lượt tải giả lập: generation + owner + lease.
	gens        map[string]int64
	owners      map[string]string
	leases      map[string]time.Time
	notifyKeys  []string
	staleClears int
}

func newMemStore() *memStore {
	return &memStore{
		channels: map[string]domain.BackdropChannel{},
		videos:   map[string]map[string]domain.BackdropChannelVideo{},
		fixed:    map[string]domain.BackdropVideo{},
		requests: map[string]backdrop.PlatformRequestRecord{},
		gens:     map[string]int64{},
		owners:   map[string]string{},
		leases:   map[string]time.Time{},
	}
}

func (m *memStore) WithTx(ctx context.Context, fn func(store backdrop.Store) error) error {
	return fn(m)
}

func (m *memStore) Channel(_ context.Context, id string) (domain.BackdropChannel, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.channels[id]
	return row, ok, nil
}

func (m *memStore) Channels(context.Context) ([]domain.BackdropChannel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.BackdropChannel, 0, len(m.channels))
	for i := len(m.channelAt) - 1; i >= 0; i-- {
		out = append(out, m.channels[m.channelAt[i]])
	}
	return out, nil
}

func (m *memStore) InsertChannel(_ context.Context, channel domain.BackdropChannel) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.channels[channel.ID]; exists {
		return false, nil
	}
	m.channels[channel.ID] = channel
	m.channelAt = append(m.channelAt, channel.ID)
	return true, nil
}

func (m *memStore) UpdateChannelList(_ context.Context, id string, now time.Time, code, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row := m.channels[id]
	if code == "" {
		row.ListedAt = &now
		row.ListErrorCode, row.ListError = "", ""
	} else {
		row.ListErrorCode, row.ListError = code, message
	}
	m.channels[id] = row
	return nil
}

func (m *memStore) UpdateChannelScan(_ context.Context, id string, nextPage *int, stopKnown bool, total int, title *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row := m.channels[id]
	row.ScanNextPage = nextPage
	row.ScanStopKnown = stopKnown
	row.ScanTotal = total
	if title != nil {
		row.Title = title
	}
	m.channels[id] = row
	return nil
}

func (m *memStore) UpdateChannelTitle(_ context.Context, id string, title string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row := m.channels[id]
	row.Title = &title
	m.channels[id] = row
	return nil
}

func (m *memStore) DeleteChannel(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.channels[id]; !ok {
		return false, nil
	}
	delete(m.channels, id)
	delete(m.videos, id)
	return true, nil
}

func (m *memStore) ChannelVideos(_ context.Context, channelID string) ([]domain.BackdropChannelVideo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rows := make([]domain.BackdropChannelVideo, 0, len(m.videos[channelID]))
	for _, row := range m.videos[channelID] {
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Kind != rows[j].Kind {
			return rows[i].Kind < rows[j].Kind
		}
		if rows[i].Position != rows[j].Position {
			return rows[i].Position < rows[j].Position
		}
		return rows[i].VideoID < rows[j].VideoID
	})
	return rows, nil
}

func (m *memStore) ChannelVideo(_ context.Context, channelID, videoID string) (domain.BackdropChannelVideo, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.videos[channelID][videoID]
	return row, ok, nil
}

func (m *memStore) UpsertVideos(_ context.Context, channelID string, videos []domain.BackdropChannelVideo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.videos[channelID] == nil {
		m.videos[channelID] = map[string]domain.BackdropChannelVideo{}
	}
	for _, video := range videos {
		if old, ok := m.videos[channelID][video.VideoID]; ok {
			if video.DurationSec == nil {
				video.DurationSec = old.DurationSec
			}
			if video.ThumbnailURL == nil {
				video.ThumbnailURL = old.ThumbnailURL
			}
			if video.PublishedAt == nil {
				video.PublishedAt = old.PublishedAt
			}
			if domain.SelfClearingBackdropReason(old.UnavailableReason) {
				video.UnavailableAt, video.UnavailableReason = nil, ""
			} else {
				video.UnavailableAt, video.UnavailableReason = old.UnavailableAt, old.UnavailableReason
			}
			video.ProbedAt, video.ProbeInfo = old.ProbedAt, old.ProbeInfo
		}
		m.videos[channelID][video.VideoID] = video
	}
	return nil
}

func (m *memStore) ExpireMissingVods(_ context.Context, channelID string, keep []string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := make(map[string]bool, len(keep))
	for _, id := range keep {
		kept[id] = true
	}
	for id, row := range m.videos[channelID] {
		if row.Kind == domain.PlatformKindVod && !kept[id] && row.UnavailableAt == nil {
			row.UnavailableAt = &now
			row.UnavailableReason = domain.BackdropExpired
			m.videos[channelID][id] = row
		}
	}
	return nil
}

func (m *memStore) MarkChannelVideoUnavailable(_ context.Context, channelID, videoID, reason string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row := m.videos[channelID][videoID]
	row.UnavailableAt, row.UnavailableReason = &now, reason
	m.videos[channelID][videoID] = row
	return nil
}

func (m *memStore) UpdateChannelVideoProbe(_ context.Context, channelID, videoID string, probe backdrop.ProbeUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row := m.videos[channelID][videoID]
	if probe.Title != nil {
		row.Title = *probe.Title
	}
	if probe.DurationSec != nil {
		row.DurationSec = probe.DurationSec
	}
	if probe.ThumbnailURL != nil {
		row.ThumbnailURL = probe.ThumbnailURL
	}
	row.ProbedAt = &probe.ProbedAt
	if probe.ClearUnavailable {
		row.UnavailableAt, row.UnavailableReason = nil, ""
	}
	m.videos[channelID][videoID] = row
	return nil
}

func (m *memStore) UsableCounts(_ context.Context, channelIDs []string) (map[string]map[domain.PlatformKind]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]map[domain.PlatformKind]int{}
	for _, id := range channelIDs {
		byKind := map[domain.PlatformKind]int{}
		for _, row := range m.videos[id] {
			if row.UnavailableAt == nil {
				byKind[row.Kind]++
			}
		}
		out[id] = byKind
	}
	return out, nil
}

func (m *memStore) PageChannelVideos(_ context.Context, query backdrop.PageQuery) ([]domain.BackdropChannelVideo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rows := make([]domain.BackdropChannelVideo, 0, len(m.videos[query.ChannelID]))
	for _, row := range m.videos[query.ChannelID] {
		if len(query.Kinds) > 0 && !containsKind(query.Kinds, row.Kind) {
			continue
		}
		if query.Cursor != nil && !afterCursor(row, query.Cursor) {
			continue
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		switch {
		case left.PublishedAt == nil && right.PublishedAt == nil:
			return left.VideoID < right.VideoID
		case left.PublishedAt == nil:
			return false
		case right.PublishedAt == nil:
			return true
		case !left.PublishedAt.Equal(*right.PublishedAt):
			return left.PublishedAt.After(*right.PublishedAt)
		default:
			return left.VideoID < right.VideoID
		}
	})
	if query.Limit > 0 && len(rows) > query.Limit {
		rows = rows[:query.Limit]
	}
	return rows, nil
}

func containsKind(kinds []string, kind domain.PlatformKind) bool {
	for _, candidate := range kinds {
		if candidate == kind {
			return true
		}
	}
	return false
}

func afterCursor(row domain.BackdropChannelVideo, cursor *backdrop.ChannelVideoCursor) bool {
	switch {
	case cursor.PublishedAt == nil:
		return row.PublishedAt == nil && row.VideoID > cursor.VideoID
	case row.PublishedAt == nil:
		return true
	case row.PublishedAt.Before(*cursor.PublishedAt):
		return true
	case row.PublishedAt.Equal(*cursor.PublishedAt):
		return row.VideoID > cursor.VideoID
	}
	return false
}

func (m *memStore) CountChannelVideos(_ context.Context, channelID string, kinds []string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for _, row := range m.videos[channelID] {
		if len(kinds) == 0 || containsKind(kinds, row.Kind) {
			total++
		}
	}
	return total, nil
}

func (m *memStore) CountUnavailableVideos(_ context.Context, channelID string, kinds []string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for _, row := range m.videos[channelID] {
		if row.UnavailableAt != nil && (len(kinds) == 0 || containsKind(kinds, row.Kind)) {
			total++
		}
	}
	return total, nil
}

func (m *memStore) Videos(context.Context) ([]domain.BackdropVideo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.BackdropVideo, 0, len(m.fixed))
	for i := len(m.fixedAt) - 1; i >= 0; i-- {
		out = append(out, m.fixed[m.fixedAt[i]])
	}
	return out, nil
}

func (m *memStore) Video(_ context.Context, id string) (domain.BackdropVideo, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.fixed[id]
	return row, ok, nil
}

func (m *memStore) InsertVideo(_ context.Context, video domain.BackdropVideo) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.fixed[video.ID]; ok {
		return false, nil
	}
	m.fixed[video.ID] = video
	m.fixedAt = append(m.fixedAt, video.ID)
	return true, nil
}

func (m *memStore) UpdateVideo(_ context.Context, id string, patch backdrop.VideoPatch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row := m.fixed[id]
	m.updateCalls++
	if patch.State != nil {
		row.State = *patch.State
	}
	if patch.ClearProgress {
		row.Progress = nil
	} else if patch.Progress != nil {
		row.Progress = patch.Progress
		m.progressWrites++
	}
	if patch.FilePath != nil {
		row.FilePath = *patch.FilePath
	}
	if patch.FileHeight != nil {
		row.FileHeight = *patch.FileHeight
	}
	if patch.FileBytes != nil {
		row.FileBytes = *patch.FileBytes
	}
	if patch.ErrorCode != nil {
		row.ErrorCode = *patch.ErrorCode
	}
	if patch.Error != nil {
		row.Error = *patch.Error
	}
	if patch.Segment != nil {
		row.Segment = *patch.Segment
	}
	if patch.Title != nil {
		row.Title = *patch.Title
	}
	if patch.DurationSec != nil {
		row.DurationSec = patch.DurationSec
	}
	row.UpdatedAt = time.Now()
	m.fixed[id] = row
	return nil
}

func (m *memStore) DeleteVideo(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.fixed[id]; !ok {
		return false, nil
	}
	delete(m.fixed, id)
	return true, nil
}

func (m *memStore) ClaimDownload(_ context.Context, id, owner string, leaseUntil, now time.Time, force bool) (int64, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.fixed[id]
	if !ok {
		return 0, false, nil
	}
	live := row.State == domain.BackdropDownloading && m.owners[id] != "" && m.leases[id].After(now)
	if live && !force {
		return 0, false, nil
	}
	m.gens[id]++
	m.owners[id] = owner
	m.leases[id] = leaseUntil
	row.State = domain.BackdropDownloading
	zero := 0.0
	row.Progress = &zero
	empty := ""
	row.ErrorCode = empty
	row.Error = empty
	m.fixed[id] = row
	return m.gens[id], true, nil
}

func (m *memStore) TouchDownload(_ context.Context, id, owner string, generation int64, progress *float64, leaseUntil, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gens[id] != generation || m.owners[id] != owner {
		return false, nil
	}
	row := m.fixed[id]
	if progress != nil {
		row.Progress = progress
		m.progressWrites++
	}
	row.UpdatedAt = now
	m.fixed[id] = row
	m.leases[id] = leaseUntil
	return true, nil
}

func (m *memStore) FinishDownload(_ context.Context, id, owner string, generation int64, filePath string, height int, size int64, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gens[id] != generation || m.owners[id] != owner {
		return false, nil
	}
	row := m.fixed[id]
	row.State = domain.BackdropReady
	row.Progress = nil
	row.FilePath = filePath
	row.FileHeight = height
	row.FileBytes = size
	row.ErrorCode = ""
	row.Error = ""
	row.UpdatedAt = now
	m.fixed[id] = row
	delete(m.owners, id)
	delete(m.leases, id)
	return true, nil
}

func (m *memStore) FailDownload(_ context.Context, id, owner string, generation int64, code, message string, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gens[id] != generation || m.owners[id] != owner {
		return false, nil
	}
	row := m.fixed[id]
	row.State = domain.BackdropFailed
	row.Progress = nil
	row.ErrorCode = code
	row.Error = message
	row.UpdatedAt = now
	m.fixed[id] = row
	delete(m.owners, id)
	delete(m.leases, id)
	return true, nil
}

func (m *memStore) ClearStaleDownloads(_ context.Context, now time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cleared := 0
	for id, row := range m.fixed {
		if row.State != domain.BackdropDownloading {
			continue
		}
		if lease, ok := m.leases[id]; ok && lease.After(now) {
			continue
		}
		row.State = domain.BackdropFailed
		row.Progress = nil
		row.ErrorCode = "interrupted"
		row.Error = "Lượt tải bị ngắt trước khi xong"
		m.fixed[id] = row
		delete(m.owners, id)
		delete(m.leases, id)
		cleared++
	}
	m.staleClears++
	return cleared, nil
}

func (m *memStore) LockResource(context.Context, string) error { return nil }

func (m *memStore) NotifyChange(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyKeys = append(m.notifyKeys, key)
	return nil
}

func (m *memStore) Templates(context.Context) ([]backdrop.TemplateRecipeRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]backdrop.TemplateRecipeRow(nil), m.templates...), nil
}

func (m *memStore) RecordPlatformRequest(_ context.Context, host, label string, now time.Time) (backdrop.PlatformRequestRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.requests[host]
	if !ok {
		record = backdrop.PlatformRequestRecord{Host: host, Label: label, FirstAt: now}
	}
	record.Count++
	record.LastAt = now
	m.requests[host] = record
	return record, nil
}

func (m *memStore) PlatformRequests(context.Context) ([]backdrop.PlatformRequestRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]backdrop.PlatformRequestRecord, 0, len(m.requests))
	for _, record := range m.requests {
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out, nil
}

// seedVideos đặt danh mục của một kênh cho test.
func (m *memStore) seedVideos(channelID string, videos ...domain.BackdropChannelVideo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.videos[channelID] == nil {
		m.videos[channelID] = map[string]domain.BackdropChannelVideo{}
	}
	for _, video := range videos {
		video.ChannelID = channelID
		m.videos[channelID][video.VideoID] = video
	}
}

func (m *memStore) seedChannel(channel domain.BackdropChannel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.channels[channel.ID]; !ok {
		m.channelAt = append(m.channelAt, channel.ID)
	}
	m.channels[channel.ID] = channel
}

func (m *memStore) seedTemplate(id, name, recipe string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.templates = append(m.templates, backdrop.TemplateRecipeRow{ID: id, Name: name, Recipe: []byte(recipe)})
}

func (m *memStore) seedFixed(video domain.BackdropVideo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.fixed[video.ID]; !ok {
		m.fixedAt = append(m.fixedAt, video.ID)
	}
	m.fixed[video.ID] = video
}

func (m *memStore) channelIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.channelAt...)
}

func (m *memStore) videoRows(channelID string) map[string]domain.BackdropChannelVideo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]domain.BackdropChannelVideo{}
	for id, row := range m.videos[channelID] {
		out[id] = row
	}
	return out
}

func (m *memStore) progressWriteCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.progressWrites
}
