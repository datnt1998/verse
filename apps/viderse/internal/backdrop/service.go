package backdrop

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
)

// PlatformState là mặt trạng thái nền tảng mà mảng này cần: giờ chờ, hạn mức
// cuộn và mốc đọc dần. Hiện thực: internal/platforms.Service; test bơm bản giả.
type PlatformState interface {
	State(ctx context.Context, id domain.PlatformID) (domain.PlatformState, bool, error)
	Budget(ctx context.Context, id domain.PlatformID) (platforms.BudgetReport, error)
	SetNextTrickle(ctx context.Context, id domain.PlatformID, at time.Time) error
}

// RunYtDlpFunc chạy một lệnh yt-dlp có nhịp cho một nền tảng; `cost` là trọng
// số đặt chỗ (một lượt trích = 4, đọc lại info đã lưu = 1). Hiện thực ở chỗ
// ráp: platforms.Pacer.RunYtDlp + tools.YtDlp.Run.
type RunYtDlpFunc func(ctx context.Context, platform domain.PlatformID, args []string, cost int, timeout time.Duration, onLine func(line string)) (string, error)

// Tasks là mặt việc nền mà mảng này cần (hiện thực: internal/activity).
// Khai interface nhỏ để gói không import activity.
type Tasks interface {
	Start(ctx context.Context, spec domain.TaskSpec) (string, error)
	Update(ctx context.Context, id string, update domain.TaskUpdate) error
	Finish(ctx context.Context, id string) error
}

// Deps là phụ thuộc của Service. Mọi mặt ngoài (lưu trữ, nhịp, HTTP, yt-dlp,
// việc nền, đồng hồ, thư mục) đều tiêm vào — test không chạm mạng.
type Deps struct {
	Store Store
	// Platforms đọc giờ chờ và hạn mức cuộn của Bilibili.
	Platforms PlatformState
	// Pacer giữ lease quét; nil = không lease (test đơn vị).
	Pacer *platforms.Pacer
	// Request là ranh giới nhịp của mọi lời gọi Bilibili (nav, trang, b23).
	Request func(ctx context.Context, resetBackoff bool, fn func(context.Context) error) error
	// Doer là HTTP thô; Service tự bọc nhịp cho lời gọi ngoài client API.
	Doer     HTTPDoer
	RunYtDlp RunYtDlpFunc
	// CfgDir chứa tệp danh tính thiết bị Bilibili (0600).
	CfgDir string
	// Cookie trả cookie đăng nhập Bilibili đã lưu (rỗng khi chưa đăng nhập).
	Cookie func(ctx context.Context) (string, error)
	Now    func() time.Time
	Random func() float64
	Tasks  Tasks
	// Spawn chạy một việc nền; nil = chạy đồng bộ (test).
	Spawn func(ctx context.Context, fn func(context.Context))
	// Background là vòng đời cho việc nền và bộ lập lịch; nil = Background().
	Background func() context.Context
	// StorageRoot trả gốc dự án (tuyệt đối).
	StorageRoot func() string
	// ResolvePath đổi đường dẫn tương đối dưới gốc dự án thành tuyệt đối.
	ResolvePath func(rel string) (string, error)
	Notify      func(ref string)
	// BilibiliPage thay client API trong test (không gọi mạng).
	BilibiliPage func(ctx context.Context, mid string, pn int) (BilibiliPage, error)
}

// Service là mặt nghiệp vụ của thư viện Video nền. An toàn cho goroutine.
type Service struct {
	d         Deps
	owner     string // chủ sở hữu lượt tải của process này
	now       func() time.Time
	random    func() float64
	wake      chan struct{}
	clientMu  sync.Mutex
	client    *bilibiliClient
	pacedDoer HTTPDoer
}

// New dựng service; thiếu phụ thuộc bắt buộc thì panic sớm ở bootstrap.
func New(d Deps) *Service {
	if d.Store == nil {
		panic("backdrop.New: thiếu Store")
	}
	if d.RunYtDlp == nil {
		panic("backdrop.New: thiếu RunYtDlp")
	}
	s := &Service{d: d, owner: newDownloadOwner(), now: d.Now, random: d.Random, wake: make(chan struct{}, 1)}
	if s.now == nil {
		s.now = time.Now
	}
	if s.random == nil {
		s.random = func() float64 { return 0.5 }
	}
	if s.d.Request == nil {
		s.d.Request = func(ctx context.Context, _ bool, fn func(context.Context) error) error { return fn(ctx) }
	}
	if s.d.Doer == nil {
		s.d.Doer = http.DefaultClient
	}
	if s.d.Cookie == nil {
		s.d.Cookie = func(context.Context) (string, error) { return "", nil }
	}
	if s.d.Background == nil {
		s.d.Background = context.Background
	}
	if s.d.Spawn == nil {
		s.d.Spawn = func(ctx context.Context, fn func(context.Context)) { fn(ctx) }
	}
	if s.d.StorageRoot == nil {
		s.d.StorageRoot = func() string { return "." }
	}
	if s.d.ResolvePath == nil {
		s.d.ResolvePath = func(rel string) (string, error) { return rel, nil }
	}
	s.pacedDoer = NewBilibiliDoer(s.d.Doer, s.d.Request, s.now)
	return s
}

// BindDeps là các mặt gắn muộn sau khi ráp: service nền tảng (giờ chờ, hạn
// mức), pacer (lease quét, nhịp), ranh giới nhịp Bilibili và cookie đăng nhập.
// Bind chạy lúc ráp, trước khi server nhận request.
type BindDeps struct {
	Platforms PlatformState
	Pacer     *platforms.Pacer
	Request   func(ctx context.Context, resetBackoff bool, fn func(context.Context) error) error
	Cookie    func(ctx context.Context) (string, error)
}

// Bind gắn các phụ thuộc nền tảng sau khi chúng được dựng (serve dựng mảng
// nền tảng sau mảng này để route POST /api/platforms/requests đăng ký trước
// tiền tố /api/platforms/).
func (s *Service) Bind(d BindDeps) {
	if d.Platforms != nil {
		s.d.Platforms = d.Platforms
	}
	if d.Pacer != nil {
		s.d.Pacer = d.Pacer
	}
	if d.Request != nil {
		s.d.Request = d.Request
		s.pacedDoer = NewBilibiliDoer(s.d.Doer, s.d.Request, s.now)
	}
	if d.Cookie != nil {
		s.d.Cookie = d.Cookie
	}
	s.clientMu.Lock()
	s.client = nil
	s.clientMu.Unlock()
	s.Wake()
}

// Wake đánh thức bộ lập lịch (có kênh mới, vừa đăng nhập, vừa gỡ chặn).
func (s *Service) Wake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Now trả giờ theo đồng hồ đã tiêm.
func (s *Service) Now() time.Time { return s.now() }

// ParseSource phân loại một link dán vào ô thêm nguồn nền.
func (s *Service) ParseSource(ctx context.Context, input string) (domain.ParsedSource, error) {
	return ParseSource(ctx, input, s.pacedDoer)
}

// bilibiliClientFor dựng client API một lần, dùng lại danh tính thiết bị.
func (s *Service) bilibiliClientFor() (*bilibiliClient, error) {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	if s.client != nil {
		return s.client, nil
	}
	client, err := newBilibiliClient(s.d.Doer, s.now, s.d.Request, newDeviceStore(s.d.CfgDir), s.d.Cookie)
	if err != nil {
		return nil, err
	}
	s.client = client
	return client, nil
}

// PlatformPause đọc giờ chờ còn hiệu lực của một nền tảng (rỗng khi không chờ).
func (s *Service) PlatformPause(ctx context.Context, id domain.PlatformID) (*time.Time, *string, error) {
	if s.d.Platforms == nil {
		return nil, nil, nil
	}
	st, _, err := s.d.Platforms.State(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return st.PausedUntil, st.PauseReason, nil
}

// Channels liệt kê mọi kênh nền kèm số đếm, trạng thái chờ và template dùng.
func (s *Service) Channels(ctx context.Context) ([]ChannelView, error) {
	rows, err := s.d.Store.Channels(ctx)
	if err != nil {
		return nil, err
	}
	templates, err := s.d.Store.Templates(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	counts, err := s.d.Store.UsableCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	var bilibili *BilibiliStatus
	if hasPlatform(rows, domain.PlatformBilibili) {
		bilibili, err = s.bilibiliStatus(ctx)
		if err != nil {
			return nil, err
		}
	}
	index := buildUsageIndex(templates)
	pauses := make(map[domain.PlatformID]pauseInfo, 2)
	for _, row := range rows {
		if _, ok := pauses[row.Platform]; ok {
			continue
		}
		until, reason, err := s.PlatformPause(ctx, row.Platform)
		if err != nil {
			return nil, err
		}
		pauses[row.Platform] = pauseInfo{until: until, reason: reason}
	}
	out := make([]ChannelView, 0, len(rows))
	for _, row := range rows {
		view, err := s.channelView(row, counts[row.ID], index, pauses[row.Platform])
		if err != nil {
			return nil, err
		}
		if row.Platform == domain.PlatformBilibili {
			view.Bilibili = bilibili
		}
		out = append(out, view)
	}
	return out, nil
}

// Channel trả một kênh nền kèm bối cảnh.
func (s *Service) Channel(ctx context.Context, id string) (ChannelView, error) {
	row, found, err := s.d.Store.Channel(ctx, id)
	if err != nil {
		return ChannelView{}, err
	}
	if !found {
		return ChannelView{}, notFoundChannel()
	}
	templates, err := s.d.Store.Templates(ctx)
	if err != nil {
		return ChannelView{}, err
	}
	counts, err := s.d.Store.UsableCounts(ctx, []string{id})
	if err != nil {
		return ChannelView{}, err
	}
	until, reason, err := s.PlatformPause(ctx, row.Platform)
	if err != nil {
		return ChannelView{}, err
	}
	view, err := s.channelView(row, counts[id], buildUsageIndex(templates), pauseInfo{until: until, reason: reason})
	if err != nil {
		return ChannelView{}, err
	}
	if row.Platform == domain.PlatformBilibili {
		view.Bilibili, err = s.bilibiliStatus(ctx)
		if err != nil {
			return ChannelView{}, err
		}
	}
	return view, nil
}

type pauseInfo struct {
	until  *time.Time
	reason *string
}

func (s *Service) channelView(row domain.BackdropChannel, counts map[domain.PlatformKind]int, index *templateUsageIndex, pause pauseInfo) (ChannelView, error) {
	if counts == nil {
		counts = map[domain.PlatformKind]int{}
	}
	normalized := make(map[domain.PlatformKind]int, len(rowKinds(row.Platform)))
	for _, kind := range rowKinds(row.Platform) {
		normalized[kind] = counts[kind]
	}
	return ChannelView{
		Channel:     row,
		Counts:      normalized,
		PausedUntil: pause.until,
		PauseReason: pause.reason,
		UsedBy:      index.channelUsages(row.ID),
	}, nil
}

func hasPlatform(rows []domain.BackdropChannel, id domain.PlatformID) bool {
	for _, row := range rows {
		if row.Platform == id {
			return true
		}
	}
	return false
}

// rowKinds là các loại video một nền tảng có, theo registry.
func rowKinds(id domain.PlatformID) []domain.PlatformKind {
	d, ok := platforms.Lookup(id)
	if !ok {
		return nil
	}
	return d.Kinds
}

// AddChannel thêm kênh từ link dán vào. Kênh đã có thì làm mới; Bilibili lưu
// trang đầu rồi đọc dần ở việc nền, Twitch đọc danh sách ngay.
func (s *Service) AddChannel(ctx context.Context, input string) (ChannelView, bool, error) {
	link, err := ParseChannelLink(ctx, input, s.pacedDoer)
	if err != nil {
		return ChannelView{}, false, err
	}
	existing, found, err := s.d.Store.Channel(ctx, link.ID())
	if err != nil {
		return ChannelView{}, false, err
	}
	if found {
		refreshed, err := s.refresh(ctx, existing, false, 0)
		if err != nil {
			return ChannelView{}, false, err
		}
		view, err := s.Channel(ctx, refreshed.ID)
		return view, false, err
	}
	// Tạo dòng trước rồi đọc danh mục ở việc nền: HTTP trả ngay, thẻ kênh hiện
	// trạng thái "đang đọc" (listError rỗng, listedAt nil) cho tới khi xong.
	queued := domain.BackdropChannel{
		ID: link.ID(), Platform: link.Platform, Handle: link.Handle, URL: link.URL,
		AddedAt: s.now(),
	}
	if link.Platform == domain.PlatformBilibili {
		queued.ScanNextPage = new(1)
	} else {
		// Danh sách phẳng Twitch không có tên hiển thị: dùng login tới khi dò
		// được tên thật.
		queued.Title = new(link.Handle)
	}
	if _, err := s.d.Store.InsertChannel(ctx, queued); err != nil {
		return ChannelView{}, false, err
	}
	s.Wake()
	s.publish(ctx, link.ID())
	s.startCatalogueScan(link)
	view, err := s.Channel(ctx, link.ID())
	return view, true, err
}

// startCatalogueScan đọc danh mục lần đầu ở việc nền, có mặt trong hàng đợi
// việc nền (backdrop_scan); lỗi được ghi vào kênh để thẻ kênh tự hiện.
func (s *Service) startCatalogueScan(link ChannelLink) {
	ctx := s.d.Background()
	s.d.Spawn(ctx, func(ctx context.Context) {
		var taskID string
		if s.d.Tasks != nil {
			taskID, _ = s.d.Tasks.Start(ctx, domain.TaskSpec{
				Kind:   domain.TaskBackdropScan,
				Title:  "Đọc danh mục Video nền: " + link.Handle,
				Object: domain.ObjectRef{Type: "backdrop_channel", ID: link.ID(), Label: link.Handle},
				Phase:  "Đang đọc danh mục video",
			})
			defer func() {
				_ = s.d.Tasks.Finish(context.WithoutCancel(ctx), taskID)
			}()
		}
		if link.Platform == domain.PlatformBilibili {
			_, _ = s.scanBilibili(ctx, link, false, false)
			return
		}
		_ = s.scanTwitch(ctx, link)
	})
}

// scanTwitch đọc danh sách Twitch vào danh mục đã có; lỗi được ghi lại trên
// kênh rồi trả nguyên lỗi cho caller (lần làm mới có yêu cầu).
func (s *Service) scanTwitch(ctx context.Context, link ChannelLink) error {
	listing, err := ListChannelVideos(ctx, s.d.RunYtDlp, link.Handle)
	if err != nil {
		_ = s.d.Store.UpdateChannelList(ctx, link.ID(), s.now(), errorCode(err), asError(err).Message)
		s.publish(ctx, link.ID())
		return err
	}
	now := s.now()
	if err := s.d.Store.WithTx(ctx, func(tx Store) error {
		if _, found, err := tx.Channel(ctx, link.ID()); err != nil {
			return err
		} else if !found {
			return notFoundChannel()
		}
		return s.ingest(ctx, tx, link.ID(), listing, now, 0)
	}); err != nil {
		return err
	}
	s.publish(ctx, link.ID())
	return nil
}

// RefreshChannel đọc lại danh sách một kênh. `force=false` bỏ qua lời gọi khi
// lần đọc thành công gần nhất còn mới.
func (s *Service) RefreshChannel(ctx context.Context, id string, force bool) (ChannelView, error) {
	channel, found, err := s.d.Store.Channel(ctx, id)
	if err != nil {
		return ChannelView{}, err
	}
	if !found {
		return ChannelView{}, notFoundChannel()
	}
	var minimumAge time.Duration
	if !force {
		minimumAge = refreshMinimumAge
	}
	refreshed, err := s.refresh(ctx, channel, force, minimumAge)
	if err != nil {
		return ChannelView{}, err
	}
	if refreshed.ID != id {
		return s.Channel(ctx, refreshed.ID)
	}
	return s.Channel(ctx, id)
}

// refreshMinimumAge là tuổi tối thiểu của lần đọc trước để một lần làm mới
// không bắt buộc được bỏ qua.
const refreshMinimumAge = 5 * time.Minute

// refresh là ruột của RefreshChannel khi đã có dòng kênh.
func (s *Service) refresh(ctx context.Context, channel domain.BackdropChannel, force bool, minimumAge time.Duration) (domain.BackdropChannel, error) {
	if minimumAge > 0 && channel.ListedAt != nil && s.now().Sub(*channel.ListedAt) < minimumAge {
		return channel, nil
	}
	link := ChannelLink{Platform: channel.Platform, Handle: channel.Handle, URL: channel.URL}
	if channel.Platform == domain.PlatformBilibili {
		// Làm mới luôn đọc trang MỚI NHẤT; con trỏ backfill lịch sử do Trickle
		// giữ riêng (newestOnly=true không đổi scan_next_page).
		return s.scanBilibili(ctx, link, true, false)
	}
	if err := s.scanTwitch(ctx, link); err != nil {
		return domain.BackdropChannel{}, err
	}
	updated, _, err := s.d.Store.Channel(ctx, channel.ID)
	return updated, err
}

// RemoveChannel xoá kênh khi không template nào còn gắn nó.
func (s *Service) RemoveChannel(ctx context.Context, id string) error {
	// Khoá transaction theo kênh: lượt lưu template lấy CÙNG khoá (xem
	// backdrop.ChannelLockKey) nên kiểm "còn template dùng" và xoá không thể
	// chen giữa lúc template vừa ghi tham chiếu.
	err := s.d.Store.WithTx(ctx, func(tx Store) error {
		if err := tx.LockResource(ctx, ChannelLockKey(id)); err != nil {
			return err
		}
		channel, found, err := tx.Channel(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return notFoundChannel()
		}
		templates, err := tx.Templates(ctx)
		if err != nil {
			return err
		}
		if using := templatesUsingChannel(templates, channel.ID); len(using) > 0 {
			names := make([]string, 0, len(using))
			for _, ref := range using {
				names = append(names, ref.Name)
			}
			return ErrorfRecovery(CodeInUse,
				fmt.Sprintf("Kênh nền đang dùng trong %d template: %s", len(using), strings.Join(names, ", ")),
				"Bỏ kênh khỏi các template này trước")
		}
		_, err = tx.DeleteChannel(ctx, id)
		return err
	})
	if err != nil {
		return err
	}
	s.publish(ctx, id)
	return nil
}

// ChannelVideosOptions là bộ lọc một trang danh mục.
type ChannelVideosOptions struct {
	Kinds  []domain.PlatformKind
	Sort   string
	Cursor string
	Limit  int
}

// ChannelVideos trả một trang danh mục của kênh, mới nhất trước.
func (s *Service) ChannelVideos(ctx context.Context, id string, opts ChannelVideosOptions) (VideoPage, error) {
	channel, found, err := s.d.Store.Channel(ctx, id)
	if err != nil {
		return VideoPage{}, err
	}
	if !found {
		return VideoPage{}, notFoundChannel()
	}
	if opts.Sort != "" && opts.Sort != "newest" {
		return VideoPage{}, ErrorfRecovery(CodeInvalidInput, "Kiểu sắp xếp không hỗ trợ", `Chỉ hỗ trợ sort=newest`)
	}
	kinds, err := normalizeKinds(channel.Platform, opts.Kinds)
	if err != nil {
		return VideoPage{}, err
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultVideoPageLimit
	}
	if limit > maxVideoPageLimit {
		limit = maxVideoPageLimit
	}
	cursor, err := decodeCursor(opts.Cursor, channel.ID, kinds)
	if err != nil {
		return VideoPage{}, err
	}
	rows, err := s.d.Store.PageChannelVideos(ctx, PageQuery{ChannelID: channel.ID, Kinds: kinds, Cursor: cursor, Limit: limit})
	if err != nil {
		return VideoPage{}, err
	}
	templates, err := s.d.Store.Templates(ctx)
	if err != nil {
		return VideoPage{}, err
	}
	chosenBy := chosenByChannel(templates, channel.ID)
	counts, err := s.d.Store.UsableCounts(ctx, []string{channel.ID})
	if err != nil {
		return VideoPage{}, err
	}
	total, err := s.d.Store.CountChannelVideos(ctx, channel.ID, kinds)
	if err != nil {
		return VideoPage{}, err
	}
	unavailable, err := s.d.Store.CountUnavailableVideos(ctx, channel.ID, kinds)
	if err != nil {
		return VideoPage{}, err
	}
	page := VideoPage{Counts: counts[channel.ID], Total: total, Unavailable: unavailable}
	if page.Counts == nil {
		page.Counts = map[domain.PlatformKind]int{}
	}
	if len(rows) == limit {
		last := rows[len(rows)-1]
		page.NextCursor = encodeCursor(ChannelVideoCursor{
			ChannelID: channel.ID, KindsKey: kindsKey(kinds),
			PublishedAt: last.PublishedAt, VideoID: last.VideoID,
		})
	}
	page.Items = make([]VideoItem, 0, len(rows))
	for _, row := range rows {
		page.Items = append(page.Items, VideoItem{Video: row, ChosenBy: chosenBy[row.VideoID]})
	}
	return page, nil
}

const (
	defaultVideoPageLimit = 24
	maxVideoPageLimit     = 500
)

// normalizeKinds kiểm bộ lọc loại thuộc nền tảng của kênh.
func normalizeKinds(platform domain.PlatformID, kinds []domain.PlatformKind) ([]string, error) {
	allowed := rowKinds(platform)
	allowedSet := make(map[domain.PlatformKind]bool, len(allowed))
	for _, kind := range allowed {
		allowedSet[kind] = true
	}
	out := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		if kind == "" {
			continue
		}
		if !allowedSet[kind] {
			return nil, ErrorfRecovery(CodeInvalidInput,
				fmt.Sprintf("Loại video %q không dùng được cho %s", kind, platform),
				"Chọn trong: "+strings.Join(allowed, ", "))
		}
		out = append(out, kind)
	}
	return out, nil
}

// ingest ghi một lượt đọc danh sách vào một transaction đã mở.
func (s *Service) ingest(ctx context.Context, tx Store, channelID string, listing ChannelListing, now time.Time, offset int) error {
	videos := make([]domain.BackdropChannelVideo, 0, len(listing.Videos))
	nextPosition := make(map[domain.PlatformKind]int, len(listing.Videos))
	for _, video := range listing.Videos {
		position, ok := nextPosition[video.Kind]
		if !ok {
			position = offset
		}
		nextPosition[video.Kind] = position + 1
		videos = append(videos, domain.BackdropChannelVideo{
			ChannelID: channelID, VideoID: video.VideoID, Kind: video.Kind, URL: video.URL,
			Title: valueOr(video.Title, ""), DurationSec: video.DurationSec,
			ThumbnailURL: video.ThumbnailURL, PublishedAt: video.PublishedAt,
			Position: position, ListedAt: now,
		})
	}
	if len(videos) > 0 {
		if err := tx.UpsertVideos(ctx, channelID, videos); err != nil {
			return err
		}
	}
	if listing.VodsComplete {
		keep := make([]string, 0, len(listing.Videos))
		for _, video := range listing.Videos {
			if video.Kind == domain.PlatformKindVod {
				keep = append(keep, video.VideoID)
			}
		}
		if err := tx.ExpireMissingVods(ctx, channelID, keep, now); err != nil {
			return err
		}
	}
	return tx.UpdateChannelList(ctx, channelID, now, "", "")
}

// ── Bilibili ───────────────────────────────────────────────────────────────

const (
	bilibiliTrickleMin    = 3 * time.Minute
	bilibiliTrickleJitter = 2 * time.Minute
)

// bilibiliStatus dựng trạng thái đọc dần: hạn mức, giờ chờ, mốc trang kế và
// các kênh còn trang chưa đọc.
func (s *Service) bilibiliStatus(ctx context.Context) (*BilibiliStatus, error) {
	now := s.now()
	status := &BilibiliStatus{}
	if s.d.Platforms != nil {
		budget, err := s.d.Platforms.Budget(ctx, domain.PlatformBilibili)
		if err != nil {
			return nil, err
		}
		status.Budget = budget
		st, _, err := s.d.Platforms.State(ctx, domain.PlatformBilibili)
		if err != nil {
			return nil, err
		}
		status.PausedUntil = st.PausedUntil
		status.PauseReason = st.PauseReason
		nextTrickle := st.NextTrickleAt
		rows, err := s.d.Store.Channels(ctx)
		if err != nil {
			return nil, err
		}
		status.Pending = pendingBilibili(rows)
		if len(status.Pending) > 0 {
			due := now
			if nextTrickle.After(due) {
				due = nextTrickle
			}
			if budget.AvailableAt.After(due) {
				due = budget.AvailableAt
			}
			if st.PausedUntil != nil && st.PausedUntil.After(due) {
				due = *st.PausedUntil
			}
			status.NextPageAt = &due
		}
	} else {
		rows, err := s.d.Store.Channels(ctx)
		if err != nil {
			return nil, err
		}
		status.Pending = pendingBilibili(rows)
		if len(status.Pending) > 0 {
			status.NextPageAt = &now
		}
		status.Budget.AvailableAt = now
	}
	return status, nil
}

func pendingBilibili(rows []domain.BackdropChannel) []PendingChannel {
	pending := make([]domain.BackdropChannel, 0, len(rows))
	for _, row := range rows {
		if row.Platform == domain.PlatformBilibili && row.ScanNextPage != nil {
			pending = append(pending, row)
		}
	}
	sort.SliceStable(pending, func(i, j int) bool {
		left, right := pending[i], pending[j]
		if (left.ListedAt == nil) != (right.ListedAt == nil) {
			return left.ListedAt == nil
		}
		if left.ListedAt != nil && !left.ListedAt.Equal(*right.ListedAt) {
			return left.ListedAt.Before(*right.ListedAt)
		}
		return left.AddedAt.Before(right.AddedAt)
	})
	out := make([]PendingChannel, 0, len(pending))
	for _, row := range pending {
		out = append(out, PendingChannel{ChannelID: row.ID, NextPage: row.ScanNextPage, MaxPages: BilibiliMaxPages})
	}
	return out
}

// scanBilibili đọc MỘT trang danh mục của một không gian Bilibili. Chỉ đọc
// trang mới nhất thì giữ nguyên con trỏ lịch sử cũ.
func (s *Service) scanBilibili(ctx context.Context, link ChannelLink, newestOnly, background bool) (domain.BackdropChannel, error) {
	scan := func(ctx context.Context) error {
		channel, found, err := s.d.Store.Channel(ctx, link.ID())
		if err != nil {
			return err
		}
		if background {
			status, err := s.bilibiliStatus(ctx)
			if err != nil {
				return err
			}
			if status.NextPageAt == nil || status.NextPageAt.After(s.now()) {
				return nil
			}
		}
		pageNumber := 1
		if !newestOnly && channel.ScanNextPage != nil {
			pageNumber = *channel.ScanNextPage
		}
		cursorKnown := channel.ScanNextPage != nil
		stopKnown := found
		if cursorKnown {
			stopKnown = channel.ScanStopKnown
		}
		page, err := s.fetchBilibiliPage(ctx, link.Handle, pageNumber)
		if err != nil {
			if found {
				_ = s.d.Store.UpdateChannelList(ctx, link.ID(), s.now(), errorCode(err), asError(err).Message)
			}
			return err
		}
		known := make(map[string]bool)
		rows, err := s.d.Store.ChannelVideos(ctx, link.ID())
		if err != nil {
			return err
		}
		for _, row := range rows {
			known[row.VideoID] = true
		}
		fullyKnown := stopKnown && len(page.Videos) > 0
		for _, video := range page.Videos {
			if !known[video.VideoID] {
				fullyKnown = false
				break
			}
		}
		reachedEnd := len(page.Videos) == 0 || pageNumber*page.PageSize >= page.Total || pageNumber >= BilibiliMaxPages
		var nextPage *int
		if !reachedEnd && !fullyKnown {
			nextPage = new(pageNumber + 1)
		}
		now := s.now()
		err = s.d.Store.WithTx(ctx, func(tx Store) error {
			current, exists, err := tx.Channel(ctx, link.ID())
			if err != nil {
				return err
			}
			if !exists {
				if found {
					return notFoundChannel()
				}
				if _, err := tx.InsertChannel(ctx, domain.BackdropChannel{
					ID: link.ID(), Platform: link.Platform, Handle: link.Handle, URL: link.URL,
					Title: page.Title, AddedAt: now,
				}); err != nil {
					return err
				}
			}
			listing := ChannelListing{Videos: page.Videos}
			if err := s.ingest(ctx, tx, link.ID(), listing, now, (pageNumber-1)*page.PageSize); err != nil {
				return err
			}
			savedNext := nextPage
			savedStop := stopKnown
			title := page.Title
			if newestOnly {
				savedNext = current.ScanNextPage
				savedStop = current.ScanStopKnown
				title = current.Title
				if page.Title != nil {
					title = page.Title
				}
			}
			if err := tx.UpdateChannelScan(ctx, link.ID(), savedNext, savedStop, page.Total, title); err != nil {
				return err
			}
			if s.d.Platforms != nil {
				nextTrickle := now.Add(bilibiliTrickleMin + time.Duration(s.random()*float64(bilibiliTrickleJitter)))
				if err := s.d.Platforms.SetNextTrickle(ctx, domain.PlatformBilibili, nextTrickle); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	}
	if s.d.Pacer != nil {
		if err := s.d.Pacer.WithScannerLease(ctx, domain.PlatformBilibili, platforms.BilibiliScanLease, scan); err != nil {
			return domain.BackdropChannel{}, err
		}
	} else if err := scan(ctx); err != nil {
		return domain.BackdropChannel{}, err
	}
	s.publish(ctx, link.ID())
	channel, _, err := s.d.Store.Channel(ctx, link.ID())
	return channel, err
}

// fetchBilibiliPage đọc một trang qua client API (hoặc hàm tiêm trong test).
func (s *Service) fetchBilibiliPage(ctx context.Context, mid string, pn int) (BilibiliPage, error) {
	if s.d.BilibiliPage != nil {
		var page BilibiliPage
		err := s.d.Request(ctx, true, func(ctx context.Context) error {
			p, err := s.d.BilibiliPage(ctx, mid, pn)
			page = p
			return err
		})
		return page, err
	}
	client, err := s.bilibiliClientFor()
	if err != nil {
		return BilibiliPage{}, err
	}
	return client.Page(ctx, mid, pn)
}

// ── Yêu cầu hỗ trợ nền tảng ────────────────────────────────────────────────

// RecordPlatformRequest ghi yêu cầu hỗ trợ một nền tảng; bấm lại không tạo
// dòng mới — chỉ tăng số lần và cập nhật lần cuối.
func (s *Service) RecordPlatformRequest(ctx context.Context, host string) (PlatformRequestRecord, error) {
	normalized := requestHost(host)
	label, unsupported := unsupportedHosts[normalized]
	if !unsupported {
		return PlatformRequestRecord{}, ErrorfRecovery(CodeUnsupportedPlatform,
			"Nền tảng này không có trong danh sách chưa hỗ trợ", "Dán link từ Douyin, TikTok, Kuaishou hoặc Xiaohongshu")
	}
	return s.d.Store.RecordPlatformRequest(ctx, normalized, label, s.now())
}

// publish phát SSE cho process này và NOTIFY cho các process khác (CLI/serve)
// đánh thức bộ lập lịch; lỗi NOTIFY không được làm hỏng thao tác.
func (s *Service) publish(ctx context.Context, ref string) {
	s.notify(ref)
	_ = s.d.Store.NotifyChange(ctx, ref)
}

// notify phát sự kiện SSE "backdrop" (web.Notifier gộp tối đa 4 frame/giây).
func (s *Service) notify(ref string) {
	if s.d.Notify != nil {
		s.d.Notify(ref)
	}
}

// requestHost nhận host trần hoặc cả link rồi trả tên miền đã chuẩn hoá.
func requestHost(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.Contains(value, "://") {
		if u, err := url.Parse(value); err == nil {
			return bareHost(u)
		}
	}
	value = strings.TrimPrefix(strings.TrimPrefix(value, "www."), "m.")
	if slash := strings.IndexByte(value, '/'); slash >= 0 {
		value = value[:slash]
	}
	return value
}

// PlatformRequests đọc mọi yêu cầu hỗ trợ đã ghi (kèm gói Báo lỗi).
func (s *Service) PlatformRequests(ctx context.Context) ([]PlatformRequestRecord, error) {
	return s.d.Store.PlatformRequests(ctx)
}

// ── Số nguồn đang dùng ─────────────────────────────────────────────────────

// Usage đếm kênh nền và video lẻ theo nền tảng cho GET /api/platforms.
func (s *Service) Usage(ctx context.Context) (map[domain.PlatformID]domain.PlatformUsage, error) {
	channels, err := s.d.Store.Channels(ctx)
	if err != nil {
		return nil, err
	}
	videos, err := s.d.Store.Videos(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.PlatformID]domain.PlatformUsage)
	for _, row := range channels {
		usage := out[row.Platform]
		usage.Channels++
		out[row.Platform] = usage
	}
	for _, row := range videos {
		usage := out[row.Platform]
		usage.Videos++
		out[row.Platform] = usage
	}
	return out, nil
}

// ── tiện ích ───────────────────────────────────────────────────────────────

func valueOr(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}
