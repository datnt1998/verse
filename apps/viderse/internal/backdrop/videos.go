package backdrop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
)

// MaxBackdropSegmentSec là trần độ dài một đoạn dùng (10 phút).
const MaxBackdropSegmentSec = 600.0

// downloadTimeout là trần một lượt tải đoạn video.
const downloadTimeout = 20 * time.Minute

// probeTimeout là trần một lượt dò metadata bằng `-J`.
const probeTimeout = 10 * time.Minute

// progressInterval là nhịp ghi tiến độ tối đa (4 lần/giây).
const progressInterval = 250 * time.Millisecond

// invalidVideoLinkMessage trùng câu của mảng nhập media khi link không phải
// một video.
const invalidVideoLinkMessage = "Dán link một video Twitch (video phát lại hoặc clip) hoặc Bilibili — không phải link kênh hay phát trực tiếp"

// Videos liệt kê mọi video lẻ kèm template đang dùng.
func (s *Service) Videos(ctx context.Context) ([]VideoView, error) {
	rows, err := s.d.Store.Videos(ctx)
	if err != nil {
		return nil, err
	}
	templates, err := s.d.Store.Templates(ctx)
	if err != nil {
		return nil, err
	}
	index := buildUsageIndex(templates)
	out := make([]VideoView, 0, len(rows))
	for _, row := range rows {
		out = append(out, VideoView{Video: row, UsedBy: index.fixedUsages(row.ID)})
	}
	return out, nil
}

// Video đọc một video lẻ kèm template đang dùng.
func (s *Service) Video(ctx context.Context, id string) (VideoView, error) {
	row, found, err := s.d.Store.Video(ctx, id)
	if err != nil {
		return VideoView{}, err
	}
	if !found {
		return VideoView{}, notFoundVideo()
	}
	templates, err := s.d.Store.Templates(ctx)
	if err != nil {
		return VideoView{}, err
	}
	return VideoView{Video: row, UsedBy: buildUsageIndex(templates).fixedUsages(row.ID)}, nil
}

// probeInfo là phần metadata dò được của một video.
type probeInfo struct {
	Title      *string  `json:"title"`
	Duration   *float64 `json:"duration"`
	Thumbnail  *string  `json:"thumbnail"`
	Uploader   *string  `json:"uploader"`
	IsLive     *bool    `json:"is_live"`
	LiveStatus *string  `json:"live_status"`
}

// AddVideo thêm một video lẻ: dò metadata, kiểm đoạn dùng rồi tải đoạn đó ở
// việc nền. Thêm lại cùng một video trả dòng đã có (tải lại khi lần trước
// hỏng).
func (s *Service) AddVideo(ctx context.Context, rawURL string, segment *domain.BackdropSegment) (VideoView, error) {
	resolved, err := s.resolveVideoLink(ctx, rawURL)
	if err != nil {
		return VideoView{}, err
	}
	id := resolved.ID()
	existing, found, err := s.d.Store.Video(ctx, id)
	if err != nil {
		return VideoView{}, err
	}
	if found {
		switch existing.State {
		case domain.BackdropReady:
			return s.Video(ctx, id)
		case domain.BackdropDownloading:
			// Process khác còn lease thì nhường; lease hết thì giành tải lại.
			s.startDownload(id, false)
			return s.Video(ctx, id)
		}
	}
	info, err := s.probeVideo(ctx, resolved)
	if err != nil {
		if found {
			code := errorCode(err)
			message := asError(err).Message
			_ = s.d.Store.UpdateVideo(ctx, id, VideoPatch{State: new(domain.BackdropFailed), ErrorCode: &code, Error: &message, ClearProgress: true})
			s.notify(id)
		}
		return VideoView{}, err
	}
	chosen, err := segmentFor(info.Duration, segment)
	if err != nil {
		return VideoView{}, err
	}
	now := s.now()
	if found {
		patch := VideoPatch{
			State: new(domain.BackdropDownloading), Progress: new(0.0), Segment: &chosen,
			Title: info.Title, DurationSec: info.Duration, ErrorCode: new(""), Error: new(""),
		}
		if err := s.d.Store.UpdateVideo(ctx, id, patch); err != nil {
			return VideoView{}, err
		}
	} else {
		title := valueOr(info.Title, id)
		row := domain.BackdropVideo{
			ID: id, Platform: resolved.Platform, URL: resolved.URL, Title: title,
			DurationSec: info.Duration, Segment: chosen,
			State: domain.BackdropDownloading, Progress: new(0.0),
			AddedAt: now, UpdatedAt: now,
		}
		if _, err := s.d.Store.InsertVideo(ctx, row); err != nil {
			return VideoView{}, err
		}
	}
	s.startDownload(id, false)
	return s.Video(ctx, id)
}

// resolveVideoLink đọc link người dùng dán, gỡ một lần chuyển hướng b23.
func (s *Service) resolveVideoLink(ctx context.Context, rawURL string) (VideoLink, error) {
	parsed, ok := parseInputURL(rawURL)
	if !ok {
		return VideoLink{}, ErrorfRecovery(CodeInvalidInput, invalidVideoLinkMessage, "")
	}
	if bareHost(parsed) == "b23.tv" {
		target, err := resolveB23(ctx, s.pacedDoer, parsed)
		if err != nil {
			return VideoLink{}, err
		}
		if target == nil {
			return VideoLink{}, ErrorfRecovery(CodeInvalidInput, invalidVideoLinkMessage, "Mở link rút gọn trong trình duyệt rồi dán địa chỉ video")
		}
		link, ok := ParseAssetLink(target.String())
		if !ok {
			return VideoLink{}, ErrorfRecovery(CodeInvalidInput, invalidVideoLinkMessage, "Địa chỉ rút gọn không trỏ tới một video Bilibili")
		}
		return link, nil
	}
	link, ok := ParseAssetLink(rawURL)
	if !ok {
		return VideoLink{}, ErrorfRecovery(CodeInvalidInput, invalidVideoLinkMessage, "")
	}
	return link, nil
}

// probeVideo dò metadata một video bằng `-J`; video đang phát trực tiếp bị
// từ chối vì chưa có bản phát lại.
func (s *Service) probeVideo(ctx context.Context, link VideoLink) (probeInfo, error) {
	stdout, err := s.d.RunYtDlp(ctx, link.Platform, []string{"-J", "--skip-download", "--no-playlist", link.URL}, 4, probeTimeout, nil)
	if err != nil {
		return probeInfo{}, err
	}
	var info probeInfo
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		return probeInfo{}, Errorf(CodeFailed, "yt-dlp trả về thông tin video không đọc được")
	}
	if (info.IsLive != nil && *info.IsLive) || (info.LiveStatus != nil && (*info.LiveStatus == "is_live" || *info.LiveStatus == "is_upcoming")) {
		return probeInfo{}, Errorf(CodeUnavailable, "Buổi phát trực tiếp chưa kết thúc — đợi có video phát lại")
	}
	return info, nil
}

// segmentFor chọn đoạn dùng: mặc định trọn video nhưng không quá 10 phút;
// đoạn người dùng đặt phải nằm trong video.
func segmentFor(duration *float64, requested *domain.BackdropSegment) (domain.BackdropSegment, error) {
	if requested != nil {
		if err := validateSegment(*requested, duration); err != nil {
			return domain.BackdropSegment{}, err
		}
		return *requested, nil
	}
	end := MaxBackdropSegmentSec
	if duration != nil && *duration > 0 && *duration < end {
		end = *duration
	}
	if end <= 0 {
		end = MaxBackdropSegmentSec
	}
	return domain.BackdropSegment{FromSec: 0, ToSec: end}, nil
}

// validateSegment kiểm ranh giới đoạn dùng so với độ dài video khi đã biết.
func validateSegment(segment domain.BackdropSegment, duration *float64) error {
	if segment.FromSec < 0 || segment.ToSec <= segment.FromSec {
		return ErrorfRecovery(CodeInvalidInput, "Đoạn dùng không hợp lệ", "Điểm cuối phải sau điểm đầu")
	}
	if segment.Duration() > MaxBackdropSegmentSec {
		return ErrorfRecovery(CodeInvalidInput,
			fmt.Sprintf("Đoạn dùng dài %.0f giây, tối đa %.0f giây", segment.Duration(), MaxBackdropSegmentSec),
			"Chọn đoạn ngắn hơn 10 phút")
	}
	if duration != nil && segment.ToSec > *duration+0.5 {
		return ErrorfRecovery(CodeInvalidInput,
			fmt.Sprintf("Đoạn dùng vượt quá độ dài video (%.0f giây)", *duration),
			"Chọn điểm cuối trong phạm vi video")
	}
	return nil
}

// UpdateVideoSegment đổi đoạn dùng và tải lại đoạn mới.
func (s *Service) UpdateVideoSegment(ctx context.Context, id string, segment domain.BackdropSegment) (VideoView, error) {
	row, found, err := s.d.Store.Video(ctx, id)
	if err != nil {
		return VideoView{}, err
	}
	if !found {
		return VideoView{}, notFoundVideo()
	}
	if err := validateSegment(segment, row.DurationSec); err != nil {
		return VideoView{}, err
	}
	progress := 0.0
	if err := s.d.Store.UpdateVideo(ctx, id, VideoPatch{
		Segment: &segment, State: new(domain.BackdropDownloading), Progress: &progress,
		ErrorCode: new(""), Error: new(""),
	}); err != nil {
		return VideoView{}, err
	}
	s.notify(id)
	s.startDownload(id, true)
	return s.Video(ctx, id)
}

// RemoveVideo xoá một video lẻ khi không template nào ghim nó trong
// fixedVideoIds.
func (s *Service) RemoveVideo(ctx context.Context, id string) error {
	var row domain.BackdropVideo
	err := s.d.Store.WithTx(ctx, func(tx Store) error {
		if err := tx.LockResource(ctx, VideoLockKey(id)); err != nil {
			return err
		}
		stored, found, err := tx.Video(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return notFoundVideo()
		}
		row = stored
		templates, err := tx.Templates(ctx)
		if err != nil {
			return err
		}
		if using := usageOfFixedVideo(templates, row.ID); len(using) > 0 {
			names := make([]string, 0, len(using))
			for _, ref := range using {
				names = append(names, ref.Name)
			}
			return ErrorfRecovery(CodeInUse,
				fmt.Sprintf("Video nền đang dùng trong %d template: %s", len(using), strings.Join(names, ", ")),
				"Bỏ video khỏi các template này trước")
		}
		_, err = tx.DeleteVideo(ctx, id)
		return err
	})
	if err != nil {
		return err
	}
	if row.FilePath != "" {
		if absolute, err := s.d.ResolvePath(row.FilePath); err == nil {
			_ = os.Remove(absolute)
		}
	}
	s.notify(id)
	return nil
}

// VideoFile trả đường dẫn tuyệt đối của tệp một video lẻ đã tải xong.
func (s *Service) VideoFile(ctx context.Context, id string) (string, error) {
	row, found, err := s.d.Store.Video(ctx, id)
	if err != nil {
		return "", err
	}
	if !found {
		return "", notFoundVideo()
	}
	if row.State != domain.BackdropReady || row.FilePath == "" {
		return "", ErrorfRecovery(CodeConflict, "Video nền chưa tải xong", "Đợi việc nền tải xong rồi mở lại")
	}
	absolute, err := s.d.ResolvePath(row.FilePath)
	if err != nil {
		return "", wrapf(err, "đường dẫn video nền")
	}
	return absolute, nil
}

// startDownload giành lượt tải rồi chạy việc nền. force=false: nhường khi
// process khác đang tải (lease còn hạn); force=true: PATCH đoạn dùng giành
// quyền, lượt cũ thấy generation lệch nên không ghi đè.
func (s *Service) startDownload(id string, force bool) {
	ctx := s.d.Background()
	now := s.now()
	generation, claimed, err := s.d.Store.ClaimDownload(ctx, id, s.owner, now.Add(downloadLease), now, force)
	if err != nil || !claimed {
		return
	}
	s.d.Spawn(ctx, func(ctx context.Context) {
		s.download(ctx, id, generation)
	})
}

// downloadLease là hạn giữ lượt tải; mỗi lần ghi tiến độ gia hạn lại.
const downloadLease = 10 * time.Minute

// newDownloadOwner sinh danh tính chủ lượt tải của process (không lộ ra wire).
func newDownloadOwner() string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("owner-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}

// download tải đúng đoạn dùng của một video ở độ cao mục tiêu của nền tảng,
// ghi tiến độ tối đa bốn lần mỗi giây, rồi đánh dấu ready hoặc failed.
func (s *Service) download(ctx context.Context, id string, generation int64) {
	row, found, err := s.d.Store.Video(ctx, id)
	if err != nil || !found {
		return
	}
	var taskID string
	if s.d.Tasks != nil {
		taskID, _ = s.d.Tasks.Start(ctx, domain.TaskSpec{
			Kind:   domain.TaskBackdropDownload,
			Title:  "Tải video nền: " + row.Title,
			Object: domain.ObjectRef{Type: "backdrop_video", ID: id, Label: row.Title},
			Phase:  "Đang tải video nền",
		})
		defer func() {
			if err := s.d.Tasks.Finish(context.WithoutCancel(ctx), taskID); err != nil {
				_ = err
			}
		}()
	}
	fail := func(cause error) {
		code := errorCode(cause)
		if code == "" {
			code = CodeFailed
		}
		message := asError(cause).Message
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		applied, _ := s.d.Store.FailDownload(cleanupCtx, id, s.owner, generation, code, message, s.now())
		if !applied {
			return
		}
		s.notify(id)
		if s.d.Tasks != nil && taskID != "" {
			_ = s.d.Tasks.Update(context.WithoutCancel(ctx), taskID, domain.TaskUpdate{State: domain.TaskFailed})
		}
	}
	link := VideoLink{Platform: row.Platform, VideoID: strings.TrimPrefix(row.ID, string(row.Platform)+":"), URL: row.URL}
	height := 480
	if descriptor, ok := platforms.Lookup(row.Platform); ok && descriptor.TargetHeight > 0 {
		height = descriptor.TargetHeight
	}
	tmp, err := os.MkdirTemp("", "viderse-backdrop-")
	if err != nil {
		fail(wrapf(err, "tạo thư mục tạm"))
		return
	}
	defer os.RemoveAll(tmp)
	fragments := "1"
	if row.Platform == domain.PlatformTwitch {
		fragments = "8"
	}
	args := []string{
		"-f", "bv*+ba/b",
		"-S", fmt.Sprintf("res:%d,vcodec:h264,acodec:aac", height),
		"--merge-output-format", "mp4",
		"--concurrent-fragments", fragments,
		"--download-sections", fmt.Sprintf("*%g-%g", row.Segment.FromSec, row.Segment.ToSec),
		"--force-keyframes-at-cuts",
		// tools.Run tự thêm --no-progress; các cờ sau bật lại tiến độ để đọc %.
		"--progress", "--newline", "--progress-template", "download:%(progress._percent_str)s",
		"-o", filepath.Join(tmp, "asset.%(ext)s"),
		link.URL,
	}
	progress := newProgressWriter(s, id, generation, ctx)
	_, err = s.d.RunYtDlp(ctx, row.Platform, args, 4, downloadTimeout, progress.line)
	if err != nil {
		fail(err)
		return
	}
	produced, err := findProduced(tmp)
	if err != nil {
		fail(err)
		return
	}
	dir := filepath.Join(s.d.StorageRoot(), "library", "backdrops")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(wrapf(err, "tạo thư mục video nền"))
		return
	}
	name := strings.ReplaceAll(id, ":", "-") + filepath.Ext(produced)
	target := filepath.Join(dir, name)
	if err := publishFile(produced, target); err != nil {
		fail(wrapf(err, "lưu video nền"))
		return
	}
	stat, err := os.Stat(target)
	if err != nil {
		fail(wrapf(err, "đọc tệp video nền"))
		return
	}
	rel := filepath.Join("library", "backdrops", name)
	heightPtr := height
	bytes := stat.Size()
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	applied, err := s.d.Store.FinishDownload(cleanupCtx, id, s.owner, generation, rel, heightPtr, bytes, s.now())
	if err != nil {
		return
	}
	if !applied {
		// Lượt bị thay thế (PATCH/xoá/process khác): không để tệp mồ côi.
		_ = os.Remove(target)
		return
	}
	s.notify(id)
}

// progressWriter ghi tiến độ tải về store, tối đa bốn lần mỗi giây và gộp
// mọi mốc (kể cả 100%) vào cùng cửa sổ: dòng lặp cùng giá trị bị bỏ, giá trị
// mới nhất chờ tới hạn rồi ghi, nên ghi DB không vượt 4 lần/giây.
type progressWriter struct {
	service    *Service
	id         string
	generation int64
	ctx        context.Context
	last       time.Time
	hasLast    bool
	lastVal    float64
	pending    *float64
}

func newProgressWriter(service *Service, id string, generation int64, ctx context.Context) *progressWriter {
	return &progressWriter{service: service, id: id, generation: generation, ctx: ctx}
}

var progressPattern = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)

func (p *progressWriter) line(line string) {
	m := progressPattern.FindStringSubmatch(line)
	if m == nil {
		return
	}
	percent, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return
	}
	value := percent / 100
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	if p.hasLast && value == p.lastVal {
		return
	}
	p.pending = &value
	if p.hasLast && p.service.now().Sub(p.last) < progressInterval {
		return
	}
	p.flush()
}

// flush ghi mốc mới nhất đang chờ; ctx của lượt tải huỷ thì ngừng ghi để
// shutdown không chờ DB.
func (p *progressWriter) flush() {
	if p.pending == nil {
		return
	}
	value := *p.pending
	p.pending = nil
	if err := p.ctx.Err(); err != nil {
		return
	}
	now := p.service.now()
	applied, _ := p.service.d.Store.TouchDownload(p.ctx, p.id, p.service.owner, p.generation, &value, now.Add(downloadLease), now)
	if !applied {
		return
	}
	p.service.notify(p.id)
	p.last = p.service.now()
	p.lastVal = value
	p.hasLast = true
}

// findProduced trả tệp media yt-dlp đã tạo trong thư mục tạm.
func findProduced(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".mp4", ".mkv", ".webm":
			return filepath.Join(dir, entry.Name()), nil
		}
	}
	return "", Errorf(CodeFailed, "yt-dlp không tạo ra file video")
}

// renameFile tách ra để test giả lập rename khác ổ đĩa.
var renameFile = os.Rename

// publishFile đưa tệp đã tải vào vị trí chính thức mà không để lại tệp dở:
// rename nguyên tử khi cùng ổ đĩa; khác ổ đĩa thì chép vào một tệp tạm cùng
// thư mục đích, sync, đóng rồi rename nguyên tử. Tệp cũ giữ nguyên tới lúc
// publish xong; mọi nhánh lỗi đều dọn tệp tạm.
func publishFile(from, to string) error {
	err := renameFile(from, to)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	destination, err := os.CreateTemp(filepath.Dir(to), ".backdrop-*.part")
	if err != nil {
		return err
	}
	staging := destination.Name()
	cleanup := func() {
		_ = destination.Close()
		_ = os.Remove(staging)
	}
	source, err := os.Open(from)
	if err != nil {
		cleanup()
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := source.Close()
	if copyErr != nil {
		cleanup()
		return copyErr
	}
	if closeErr != nil {
		cleanup()
		return closeErr
	}
	if err := destination.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := destination.Close(); err != nil {
		_ = os.Remove(staging)
		return err
	}
	if err := renameFile(staging, to); err != nil {
		_ = os.Remove(staging)
		return err
	}
	return os.Remove(from)
}
