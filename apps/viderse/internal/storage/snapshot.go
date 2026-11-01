package storage

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Dung lượng thư mục dự án KHÔNG được đi cây trong request: GET /api/storage
// chỉ đọc bộ nhớ. Cây được đo một lần lúc mở và chỉ đo lại khi có người báo
// đổi (move/dọn/đổi vị trí) hoặc người dùng bấm check — idle thì không đi
// cây gì cả. Có giới hạn số mục và dừng theo ctx.
const (
	usageWalkLimit = 500_000 // số tệp/thư mục tối đa một lần đo
)

// usageSnapshot là số đo dung lượng một lần.
type usageSnapshot struct {
	used   int64
	groups []UsageGroup
	stay   int64
}

// usageOf trả số đo đang có trong bộ nhớ (không đi cây).
func (s *Service) usageOf() usageSnapshot {
	s.usageMu.Lock()
	defer s.usageMu.Unlock()
	return usageSnapshot{used: s.usageUsed, groups: append([]UsageGroup(nil), s.usageList...), stay: s.stayBytes}
}

// markUsageDirty báo số đo đã cũ; vòng nền sẽ đo lại (không đo trong request).
func (s *Service) markUsageDirty() {
	s.usageMu.Lock()
	s.usageDirty = true
	s.usageMu.Unlock()
	select {
	case s.usageWake <- struct{}{}:
	default:
	}
}

// refreshUsage đo lại cây dự án và thư mục luôn-ở-máy, có giới hạn và ctx.
// Chạy tuần tự (usageWalkMu) để hai lần đo không chồng nhau.
func (s *Service) refreshUsage(ctx context.Context) {
	if s.RemoteStorage() {
		return
	}
	s.usageWalkMu.Lock()
	defer s.usageWalkMu.Unlock()
	root := s.Root()
	snap := measureTree(ctx, root)
	snap.stay = measureStay(ctx, s.deps.ConfigDir, s.deps.DefaultDir)
	s.usageMu.Lock()
	s.usageUsed, s.usageList, s.stayBytes = snap.used, snap.groups, snap.stay
	s.usageAt, s.usageDirty = s.clock.Now(), false
	s.usageMu.Unlock()
}

// measureTree cộng dung lượng theo nhóm một thư mục dự án, dừng theo ctx và
// giới hạn số mục (cây khổng lồ không được làm treo vòng nền).
func measureTree(ctx context.Context, root string) usageSnapshot {
	snap := usageSnapshot{}
	if root == "" {
		return snap
	}
	groups := map[Group]int64{}
	var other int64
	count := 0
	entries, err := os.ReadDir(root)
	if err != nil {
		return snap
	}
	for _, e := range entries {
		if e.Name() == RootMarker || e.Name() == stagingDirName {
			continue
		}
		var bytes int64
		bytes, count = treeBytes(ctx, filepath.Join(root, e.Name()), count)
		if g := Group(e.Name()); isGroup(g) {
			groups[g] += bytes
		} else {
			other += bytes
		}
		if ctx.Err() != nil || count >= usageWalkLimit {
			break
		}
	}
	for _, g := range Groups {
		if groups[g] > 0 {
			snap.groups = append(snap.groups, UsageGroup{ID: g, Bytes: groups[g]})
			snap.used += groups[g]
		}
	}
	if other > 0 {
		snap.groups = append(snap.groups, UsageGroup{ID: "other", Bytes: other})
		snap.used += other
	}
	return snap
}

// treeBytes cộng kích thước một tệp/cây, kiểm ctx mỗi 256 mục; trả tổng và
// số mục đã đếm (để caller dừng khi vượt giới hạn).
func treeBytes(ctx context.Context, path string, count int) (int64, int) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, count
	}
	count++
	if !info.IsDir() {
		return info.Size(), count
	}
	var total int64
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, count
	}
	for i, e := range entries {
		if i%256 == 0 && ctx.Err() != nil {
			return total, count
		}
		var bytes int64
		bytes, count = treeBytes(ctx, filepath.Join(path, e.Name()), count)
		total += bytes
		if count >= usageWalkLimit {
			return total, count
		}
	}
	return total, count
}

// measureStay đo phần luôn ở máy này (config dir trừ thư mục dự án mặc định),
// có giới hạn + ctx.
func measureStay(ctx context.Context, configDir, projects string) int64 {
	if configDir == "" {
		return 0
	}
	projects = filepath.Clean(projects)
	var total int64
	count := 0
	_ = filepath.Walk(configDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && filepath.Clean(path) == projects {
			return filepath.SkipDir
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		count++
		if ctx.Err() != nil || count >= usageWalkLimit {
			return context.Canceled
		}
		return nil
	})
	return total
}

// boundedBytes đo tổng dung lượng một cây với giới hạn thời gian — dùng cho
// bản cũ của "chỉ video mới" (có thể rất lớn nhưng không được treo request).
func boundedBytes(ctx context.Context, root string) int64 {
	walkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	total, _ := treeBytes(walkCtx, root, 0)
	return total
}

func isGroup(g Group) bool {
	for _, k := range Groups {
		if k == g {
			return true
		}
	}
	return false
}

// suggestions trả gợi ý thư mục trên các ổ đang cắm (thư mục con "Viderse
// Projects" của từng ổ).
func (s *Service) suggestions(ctx context.Context) []Suggestion {
	vols, err := s.deps.Volumes.List(ctx)
	if err != nil {
		return nil
	}
	out := make([]Suggestion, 0, len(vols))
	for _, v := range vols {
		if v.Kind == VolumeCloudSync {
			continue
		}
		path := filepath.Join(v.Mount, ProjectsFolderName)
		if v.Kind == VolumeInternal {
			path = s.deps.DefaultDir
		}
		out = append(out, Suggestion{Path: path, Volume: v})
	}
	return out
}

// Snapshot dựng hình GET /api/storage từ bộ nhớ: không đi cây, không gọi
// diskutil (Volumes dùng cache), không truy vấn DB ngoài settings đã có.
func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	move := s.MoveInfo()
	root := s.Root()
	usage := s.usageOf()
	vol, _ := s.deps.Volumes.Info(ctx, root)
	offline, since := s.Offline()
	if offline {
		// Ổ đang giữ thư mục dự án không đọc được: giữ tên ổ thật, không lấy
		// chỗ trống của ổ khác (#79).
		vol = s.offlineVolume(ctx, root)
	}

	var prev *domain.StoragePrevious
	s.mu.Lock()
	if s.previous != nil {
		copy := *s.previous
		prev = &copy
	}
	s.mu.Unlock()

	var lastSwitch *domain.StorageSwitch
	var stored domain.StorageSwitch
	if found, err := s.deps.Settings.Get(ctx, domain.KeyStorageLastSwitch, &stored); err == nil && found && stored.Mode != "" {
		copy := stored
		lastSwitch = &copy
	}

	var lastRun *time.Time
	var raw string
	if found, err := s.deps.Settings.Get(ctx, domain.KeyStorageCleanAt, &raw); err == nil && found {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			lastRun = &t
		}
	}

	return Snapshot{
		Projects: Location{
			Path: root, Volume: vol, UsedBytes: usage.used,
			Online: !offline, OfflineSince: since, Previous: prev, LastSwitch: lastSwitch,
		},
		StayBytes:    usage.stay,
		Suggestions:  s.suggestions(ctx),
		Usage:        usage.groups,
		CleanLastRun: lastRun,
		Move:         move,
	}, nil
}

// Changed báo dữ liệu vừa đổi: vòng nền đo lại rồi mới phát sự kiện, để client
// tải lại GET /api/storage là thấy đúng số mới (không phát trước khi đo).
func (s *Service) Changed(ref string) {
	s.usageMu.Lock()
	s.usageDirty = true
	s.pendingRefs = append(s.pendingRefs, ref)
	s.usageMu.Unlock()
	select {
	case s.usageWake <- struct{}{}:
	default:
	}
}

// refreshUsageLoop chỉ đo lại khi có báo đổi (wake) — idle hoàn toàn nằm yên,
// đúng ngân sách idle: SSE ping + stat của watch ổ.
func (s *Service) refreshUsageLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.usageWake:
			s.refreshUsage(ctx)
			s.flushChanged()
		}
	}
}

// flushChanged phát các sự kiện storage đang chờ, sau khi số đo đã mới.
func (s *Service) flushChanged() {
	s.usageMu.Lock()
	refs := s.pendingRefs
	s.pendingRefs = nil
	s.usageMu.Unlock()
	if len(refs) == 0 {
		s.emit("")
		s.bump()
		return
	}
	for _, ref := range refs {
		s.emit(ref)
		s.bump()
	}
}

// forceUsage đo lại ngay (dùng cho check trước khi chuyển — thao tác người
// dùng, không phải hot path), có giới hạn thời gian để không treo request.
func (s *Service) forceUsage(ctx context.Context) {
	walkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	s.refreshUsage(walkCtx)
	if walkCtx.Err() != nil {
		slog.Warn("storage: đo dung lượng chạm giới hạn thời gian; dùng số đã có")
	}
}
