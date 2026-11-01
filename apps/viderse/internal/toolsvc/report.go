package toolsvc

import (
	"context"
	"path/filepath"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/toolchain"
)

// StepChecks là đánh giá sáu bước Chuẩn bị máy theo nhu cầu của bản cũ:
// install = app tự sửa, login = chờ người dùng, fix = hỏng cần xử lý.
// fresh=false đọc snapshot cache; hàng đang cài báo running ngoài chỗ này.
func (s *Service) StepChecks(ctx context.Context, fresh bool) (map[domain.SetupStepID]domain.SetupCheck, error) {
	snap, err := s.Snapshot(ctx, fresh)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.SetupStepID]domain.SetupCheck, len(snap.Tools))
	for _, row := range snap.Tools {
		out[row.ID] = rowCheck(row)
	}
	return out, nil
}

func rowCheck(row Row) domain.SetupCheck {
	out := domain.SetupCheck{SizeBytes: row.SizeBytes, Provided: row.Provided}
	switch row.State {
	case StateReady, StateUpdate:
		out.Need = domain.SetupNeedReady
	case StateNeedsLogin:
		out.Need = domain.SetupNeedLogin
	case StateInstalling:
		// Lượt cài đang chạy: vẫn thiếu — setup báo running, không phải done.
		out.Need = domain.SetupNeedInstall
	default:
		out.Need = domain.SetupNeedInstall
	}
	return out
}

// LiveProgress là tiến độ trực tiếp của lượt cài (nil khi không còn chạy).
func (s *Service) LiveProgress(id ToolID) *domain.InstallProgress {
	return liveProgress(s.mgr, id)
}

// RequiredBytes là dung lượng trống tối thiểu để cài một bước — chỉ cho
// kiểm tra đĩa, KHÔNG BAO GIỜ hiện ra UI (nó là chỗ dự phòng, lớn gấp 2–4
// lần kích thước cài thật).
func (s *Service) RequiredBytes(id ToolID) int64 {
	switch id {
	case ToolYtDlp:
		return tools.YtDlpRequiredBytes
	case ToolFfmpeg:
		return toolchain.FfmpegPin().RequiredBytes
	case ToolTranscription:
		return tools.PythonEnvRequiredBytes
	case ToolChrome:
		return tools.ChromePin().RequiredBytes
	case ToolCodex:
		return tools.CodexPin().RequiredBytes
	}
	return 0
}

// ApproxBytes là dung lượng cài đặt xấp xỉ (đo trên máy thật) để hàng chưa
// cài và phần giới thiệu “khoảng X GB” có con số thật — không phải chỗ dành
// cho ổ đĩa.
func (s *Service) ApproxBytes(id ToolID) int64 {
	switch id {
	case ToolYtDlp:
		return tools.YtDlpApproxBytes
	case ToolFfmpeg:
		return toolchain.FfmpegPin().ApproxBytes
	case ToolTranscription:
		return tools.PythonEnvApproxBytes
	case ToolChrome:
		return tools.ChromePin().ApproxBytes
	case ToolCodex:
		return tools.CodexPin().ApproxBytes
	}
	return 0
}

// installedSizeBytes là dung lượng đã đo và ghi lại sau một lượt cài (0 khi
// chưa cài qua app, chưa đo, hoặc tệp hỏng — UI hiện “Đã cài” không số).
func (s *Service) installedSizeBytes(id ToolID) int64 {
	if s.dataDir == "" {
		return 0
	}
	return toolchain.ReadInstalledSize(s.dataDir, string(id))
}

// sizeForRow chọn dung lượng của một hàng theo trạng thái:
//   - provided → 0 (app không tải, hiện “Có sẵn”).
//   - ready/update → số đã đo sau lượt cài; 0 nếu không đo được (cài từ
//     trước tính năng đo) — không bịa con số.
//   - mọi trạng thái chưa xong → xấp xỉ đo từ manifest.
func (s *Service) sizeForRow(id ToolID, row Row) int64 {
	if row.Provided {
		return 0
	}
	switch row.State {
	case StateReady, StateUpdate:
		return s.installedSizeBytes(id)
	}
	return s.ApproxBytes(id)
}

// measureInstalled đo dung lượng cài đặt thật của một công cụ và ghi lại.
// Chỉ gọi trong tác vụ cài, sau khi installer báo xong — đi qua cây thư mục
// nên KHÔNG chạy trong đường GET.
func (s *Service) measureInstalled(id ToolID) int64 {
	if s.dataDir == "" {
		return 0
	}
	var total int64
	switch id {
	case ToolYtDlp:
		total = toolchain.DirSizeBytes(filepath.Join(s.dataDir, "bin", "yt-dlp"))
	case ToolFfmpeg, ToolChrome, ToolCodex:
		total = toolchain.DirSizeBytes(toolchain.ActiveVersionDir(s.dataDir, string(id)))
	case ToolTranscription:
		// env + models HF + interpreter quản lý; bỏ cache/uv (cache, không
		// phải phần cài). Đo cả thư mục python/ (gồm transcript-requirements
		// vài KB, không đáng kể) thay vì đoán tên cpython-*.
		total += toolchain.DirSizeBytes(filepath.Join(s.dataDir, "envs", "transcription"))
		total += toolchain.DirSizeBytes(filepath.Join(s.dataDir, "models", "hf"))
		total += toolchain.DirSizeBytes(filepath.Join(s.dataDir, "python"))
	default:
		return 0
	}
	if total > 0 {
		if err := toolchain.WriteInstalledSize(s.dataDir, string(id), total); err != nil {
			s.logf(id, "warn", "ghi số đo dung lượng: %v", err)
		}
	}
	return total
}

// Attention là các mục cần xử lý của status nguồn tools: bản cập nhật yt-dlp
// và lượt cài/hàng thất bại — chấm ở Cài đặt › Công cụ trên máy.
func (s *Service) Attention(ctx context.Context) []Attention {
	snap, err := s.Snapshot(ctx, false)
	if err != nil {
		return nil
	}
	var out []Attention
	for _, row := range snap.Tools {
		switch row.State {
		case StateUpdate:
			out = append(out, Attention{
				Kind: domain.AttentionToolUpdate,
				Text: "Có bản mới cho Tải video và phụ đề",
				Href: "#/settings?section=tools",
			})
		case StateFailed:
			out = append(out, Attention{
				Kind: domain.AttentionToolFailed,
				Text: installTitle(row.ID) + " chưa cài được",
				Href: "#/settings?section=tools",
			})
		}
	}
	return out
}

// Attention là một mục cần xử lý trên chấm bánh răng — hình chung với
// status.Attention nhưng khai ở đây để gói này không import internal/status
// (adapter ở cmd/viderse).
type Attention struct {
	Kind string
	Text string
	Href string
}
