package assistant

import (
	"context"

	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	clonebackdrop "github.com/datnt1998/verse/apps/viderse/internal/clone/backdrop"
	cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
	clonemetadata "github.com/datnt1998/verse/apps/viderse/internal/clone/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/rivals"
	"github.com/datnt1998/verse/core/llmretry"
)

// Các interface dưới đây là mặt nhỏ của MỖI service mà giao diện đang dùng:
// trợ lý không có đường chạy riêng. Mỗi `run` gọi đúng hàm mà nút bấm tương
// ứng gọi (xem bảng intent → service ở intents.go).

// Jobs đọc video đang làm (kênh chủ, tiêu đề nguồn) để dựng phạm vi và thẻ.
type Jobs interface {
	Get(ctx context.Context, id string) (*domain.CloneJobView, error)
	Pause(ctx context.Context, id string) (*domain.CloneJob, error)
	Resume(ctx context.Context, id string) (*domain.CloneJob, error)
}

// Images là mặt ảnh của một video (cùng service với màn Duyệt ảnh).
type Images interface {
	Read(ctx context.Context, id string) (cloneimages.View, error)
	Review(ctx context.Context, id string, in cloneimages.ReviewInput) (cloneimages.ReviewResult, error)
	Redraw(ctx context.Context, id, itemID string, in cloneimages.RedrawInput) (cloneimages.GenerateResult, error)
	RedrawAll(ctx context.Context, id string, in cloneimages.ResetInput) (cloneimages.GenerateResult, error)
	PlanOverrides(ctx context.Context, id string, in cloneimages.OverrideInput) (cloneimages.OverridePlan, error)
	ApplyOverrides(ctx context.Context, id string, in cloneimages.OverrideInput) (cloneimages.OverridePlan, error)
}

// Backdrops là service nền của video (Đổi nền khác).
type Backdrops interface {
	Edit(ctx context.Context, id string, in clonebackdrop.EditRequest) (clonebackdrop.Result, error)
}

// Metadata là service tiêu đề/mô tả/tags của video.
type Metadata interface {
	Read(ctx context.Context, id string) (clonemetadata.View, error)
	Rewrite(ctx context.Context, id string, in clonemetadata.Rewrite) (clonemetadata.View, error)
}

// Channels là service kênh của chủ (nhịp, bộ lọc Tự lấy theo đối thủ).
type Channels interface {
	List(ctx context.Context) ([]channels.ChannelView, error)
	Get(ctx context.Context, id string) (channels.ChannelView, error)
	Update(ctx context.Context, id string, in channels.UpdateRequest) (channels.ChannelView, error)
	SetSourceMinDuration(ctx context.Context, id, rivalID string, minDurationSec *int) (channels.ChannelView, error)
}

// Queue là hàng chờ làm video (cùng service với Quy tắc chạy và Thêm video).
type Queue interface {
	List(ctx context.Context) (domain.CloneQueueView, error)
	Plan(ctx context.Context, in domain.CloneEnqueueRequest) (domain.ClonePlan, error)
	Enqueue(ctx context.Context, in domain.CloneEnqueueRequest) (domain.CloneEnqueueResult, error)
	Patch(ctx context.Context, in domain.QueuePatch) (domain.CloneQueueView, error)
}

// Rivals đọc video của đối thủ đã theo dõi.
type Rivals interface {
	Videos(ctx context.Context, channelID string, in rivals.VideoQuery) (*rivals.VideoPage, error)
}

// Autonomy là service Mức tự chủ của kênh. Trợ lý chỉ đặt ba mức có sẵn
// (careful | balanced | auto), không bao giờ đặt "Tuỳ chỉnh".
type Autonomy interface {
	ApplyPreset(ctx context.Context, channelID, preset string) error
}

// Rules là service Quy tắc đã học của kênh.
type Rules interface {
	AddRule(ctx context.Context, channelID, text, source string) (ruleID string, err error)
}

// Tasks là việc nền (activity) mà tiến độ run-plan hiện lên.
type Tasks interface {
	Start(ctx context.Context, spec domain.TaskSpec) (string, error)
	Update(ctx context.Context, id string, u domain.TaskUpdate) error
	Finish(ctx context.Context, id string) error
}

// Deps là mọi thứ Service cần. Director trả model Đạo diễn hiện tại (hoặc lỗi
// khi chưa đăng nhập) — mỗi lần gọi đọc lại, không giữ client cũ.
type Deps struct {
	Director  func(ctx context.Context) (llmretry.Generator, error)
	Jobs      Jobs
	Images    Images
	Backdrops Backdrops
	Metadata  Metadata
	Channels  Channels
	Queue     Queue
	Rivals    Rivals
	Autonomy  Autonomy
	Rules     Rules
	Tasks     Tasks
	// ImagesLeft trả số ảnh còn lại hôm nay; nil hoặc lỗi = không báo.
	ImagesLeft func(ctx context.Context) (int, error)
	// Diag nhận lỗi nội bộ không hiện cho người dùng.
	Diag func(error)
}
