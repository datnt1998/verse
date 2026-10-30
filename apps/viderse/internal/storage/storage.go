// Package storage quản lý thư mục lưu dự án của Viderse (hợp đồng dữ liệu
// §23, Cài đặt › Lưu trữ): nơi đặt thư mục, resolver cho đường dẫn tương đối
// trong DB, kiểm tra trước khi chuyển, việc chuyển nền có huỷ/phục hồi, theo
// dõi ổ bị rút và các hành động trên bản cũ.
//
// Layout của thư mục dự án (phase sau ghi, package này định nghĩa và dọn):
//
//	{root}/
//	├── images/                       # ảnh cảnh/ảnh bìa đã vẽ (một thư mục con theo video)
//	├── editor/                       # workspace editor
//	│   ├── projects/<id>/            # project.json + JSON cạnh bên + exports/
//	│   ├── media/<id>/               # metadata.json + tệp nguồn
//	│   └── media-reclaim-pending.json
//	├── sources/<videoId>/            # video gốc tải từ đối thủ + evidence nhỏ
//	└── exports/                      # MP4 và bằng chứng xuất
//	    ├── <videoId>/                # bản xuất của một video
//	    ├── requests/<id>.json        # bản ghi yêu cầu xuất (render headless)
//	    └── profiles/<id>/            # profile Chrome của một yêu cầu
//
// Mọi đường dẫn lưu trong DB là tương đối với thư mục này — đổi thư mục không
// cần khởi động lại và không phải sửa DB.
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Group là một nhóm thư mục con cấp một của thư mục dự án — đơn vị báo cáo
// tiến độ chuyển (§23) và phần dung lượng.
type Group = string

const (
	GroupImages  Group = "images"
	GroupEditor  Group = "editor"
	GroupSources Group = "sources"
	GroupExports Group = "exports"
)

// Groups là thứ tự cố định của các nhóm khi chuyển và khi báo cáo.
var Groups = []Group{GroupImages, GroupEditor, GroupSources, GroupExports}

// VolumeKind là loại ổ chứa thư mục (§23).
type VolumeKind = string

const (
	VolumeInternal  VolumeKind = "internal"
	VolumeExternal  VolumeKind = "external"
	VolumeNetwork   VolumeKind = "network"
	VolumeCloudSync VolumeKind = "cloud_sync"
)

// CheckKind là loại kiểm tra trong POST /api/storage/check (§23).
type CheckKind = string

const (
	// CheckPath: đích không phải chỗ hợp lệ (lồng thư mục hiện tại, vùng hệ
	// thống, hoặc đích không trống mà không phải thư mục dự án của Viderse).
	CheckPath       CheckKind = "path"
	CheckSpace      CheckKind = "space"
	CheckWrite      CheckKind = "write"
	CheckCloud      CheckKind = "cloud"
	CheckFS         CheckKind = "fs"
	CheckSameVolume CheckKind = "same_volume"
)

// Volume là thông tin của ổ chứa một đường dẫn.
type Volume struct {
	Name      string
	Kind      VolumeKind
	FS        string // tên hiển thị: APFS, exFAT, FAT32, SMB…
	FSType    string // tên statfs thô: apfs, exfat, msdos, smbfs…
	FreeBytes int64
	// Bus là cổng nối của ổ ngoài theo diskutil (USB, Thunderbolt…); rỗng với
	// ổ trong máy, ổ mạng hoặc khi chưa phân loại được.
	Bus   string
	Mount string // điểm gắn (canonical)

	id [2]int32 // fsid — định danh filesystem, so "cùng ổ"
}

// fsid trả định danh filesystem (so "cùng ổ").
func (v Volume) fsid() [2]int32 { return v.id }

// CheckStatus là kết quả của một kiểm tra.
type CheckStatus = string

const (
	CheckOK   CheckStatus = "ok"
	CheckWarn CheckStatus = "warn"
	CheckFail CheckStatus = "fail"
)

// Check là một dòng kiểm tra của /api/storage/check.
type Check struct {
	Kind   CheckKind
	Status CheckStatus
	Text   string
}

// MoveMode là cách xử lý dữ liệu đang có khi đổi thư mục (§23).
type MoveMode = string

const (
	// ModeMove chuyển toàn bộ dữ liệu sang thư mục mới (mặc định).
	ModeMove MoveMode = "move"
	// ModeNewOnly chỉ video mới lưu ở thư mục mới; dự án cũ ở lại và vẫn đọc
	// được qua altRoots.
	ModeNewOnly MoveMode = "new_only"
)

// CheckResult là kết quả của POST /api/storage/check.
type CheckResult struct {
	OK           bool
	Path         string // đích thật sau khi resolve (đã gồm subdir nếu có)
	CreateSubdir bool   // chọn gốc ổ → server tạo "Viderse Projects"
	Checks       []Check
	NeedBytes    int64
	ETASec       *int
}

// Location là thông tin thư mục dự án hiện tại (§23 StorageLocation).
type Location struct {
	Path         string
	Volume       Volume
	UsedBytes    int64
	Online       bool
	OfflineSince *time.Time
	Previous     *domain.StoragePrevious
	// LastSwitch là lần đổi thư mục gần nhất — UI phân biệt màn "đã chuyển
	// xong" với "đã đổi cho video mới" sau khi tải lại.
	LastSwitch *domain.StorageSwitch
}

// Suggestion là một gợi ý thư mục trên ổ đang cắm.
type Suggestion struct {
	Path   string
	Volume Volume
}

// UsageGroup là phần dung lượng của một nhóm trong thư mục dự án.
type UsageGroup struct {
	ID    Group
	Bytes int64
}

// Snapshot là hình dữ liệu của GET /api/storage.
type Snapshot struct {
	Projects    Location
	StayBytes   int64 // cơ sở dữ liệu, mô hình, công cụ — luôn ở máy này
	Suggestions []Suggestion
	// Move là trạng thái lần đổi thư mục: đang chạy và lần hỏng gần nhất.
	Move         MoveInfo
	Usage        []UsageGroup
	CleanLastRun *time.Time
}

// BrowseEntry là một thư mục trong hộp chọn phía server (hỏi đáp #10).
type BrowseEntry struct {
	Name string
	Path string
}

// BrowseResult là kết quả của GET /api/storage/browse.
type BrowseResult struct {
	Path  string
	Roots []BrowseEntry // thư mục nhà + các ổ đang cắm (luôn có)
	Dirs  []BrowseEntry
}

/* ------------------------------- interfaces ------------------------------- */

// Settings là phần kho khoá/giá trị storage cần (postgres.SettingsStore thoả).
type Settings interface {
	Get(ctx context.Context, key string, out any) (bool, error)
	Set(ctx context.Context, key string, value any) error
	Clear(ctx context.Context, key string) error
}

// Volumes là nguồn thông tin ổ đĩa (statfs + diskutil trên darwin). List trả
// các ổ đang gắn hiển thị được; Info trả ổ chứa path.
type Volumes interface {
	List(ctx context.Context) ([]Volume, error)
	Info(ctx context.Context, path string) (Volume, error)
}

// Opener mở một thư mục trong trình quản lý tệp của máy chạy Viderse
// (`open` trên darwin). Tiêm bản giả trong test.
type Opener interface {
	Open(ctx context.Context, path string) error
}

// Tasks là việc nền (contract chung; activity.Service thoả).
type Tasks interface {
	Start(ctx context.Context, spec domain.TaskSpec) (string, error)
	Update(ctx context.Context, id string, u domain.TaskUpdate) error
	Finish(ctx context.Context, id string) error
}

// Notifications là bộ ghi thông báo (contract chung; activity.Service thoả).
type Notifications interface {
	Notify(ctx context.Context, kind, text, href, actionLabel string) (string, error)
}

// Clock cho phép test điều khiển thời gian.
type Clock interface {
	Now() time.Time
}

// Ticker là nguồn nhịp — test đẩy từng nhịp bằng tay.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type realTicker struct{ t *time.Ticker }

func (r realTicker) C() <-chan time.Time { return r.t.C }
func (r realTicker) Stop()               { r.t.Stop() }

/* --------------------------------- errors --------------------------------- */

// Error là lỗi nghiệp vụ của storage có mã ổn định để web dịch sang câu.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// Mã lỗi của mảng storage (trường "code" của body lỗi).
const (
	ErrInvalidPath     = "invalid_path"
	ErrNotFound        = "not_found"
	ErrFolderMissing   = "missing"
	ErrCloudSync       = "cloud_sync"
	ErrMoveInProgress  = "move_in_progress"
	ErrNoMove          = "no_move"
	ErrVolumeGone      = "volume_gone"
	ErrUnknownReclaim  = "unknown_kind"
	ErrPreviewRequired = "preview_required"
	// ErrNotDeletable: bản cũ của "chỉ video mới" vẫn là dữ liệu đang dùng.
	ErrNotDeletable = "not_deletable"
	// ErrNotSupported: máy chủ không có trình mở thư mục (không phải macOS
	// hay Linux có xdg-open).
	ErrNotSupported = "not_supported"
	// ErrDestNotEmpty: đích không trống và không phải thư mục dự án Viderse.
	ErrDestNotEmpty = "dest_not_empty"
	// ErrDestConflict: đích đã có tệp khác nội dung tại đường dẫn sẽ ghi —
	// từ chối thay vì ghi đè dữ liệu người dùng.
	ErrDestConflict = "dest_conflict"
)

// CodeOf trích mã lỗi ổn định; lỗi lạ → "" (web trả 500).
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func codeError(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}
