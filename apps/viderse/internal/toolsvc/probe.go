package toolsvc

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/uploadlimit"
	"github.com/datnt1998/verse/platform/toolchain"
)

// probe đánh giá mọi công cụ một lượt: các probe chạy song song (spawn
// process có timeout riêng bên trong), hàng giữ đúng thứ tự của design.
func (s *Service) probe(ctx context.Context) Snapshot {
	rows := make([]Row, len(ToolIDs))
	var wg sync.WaitGroup
	for i, id := range ToolIDs {
		wg.Add(1)
		go func(i int, id ToolID) {
			defer wg.Done()
			rows[i] = s.probeOne(ctx, id)
		}(i, id)
	}
	wg.Wait()
	for i, id := range ToolIDs {
		rows[i].SizeBytes = s.sizeForRow(id, rows[i])
	}
	return Snapshot{Tools: rows, CheckedAt: s.now()}
}

// probeOne đánh giá một hàng: lượt cài đang chạy thắng mọi trạng thái probe.
func (s *Service) probeOne(ctx context.Context, id ToolID) Row {
	s.mu.Lock()
	run, installing := s.running[id]
	s.mu.Unlock()
	if installing && run.alive() {
		return Row{ID: id, State: StateInstalling, Hint: "Đang cài…",
			Progress: liveProgress(s.mgr, id)}
	}
	switch id {
	case ToolYtDlp:
		return s.probeYtDlp(ctx)
	case ToolFfmpeg:
		return s.probeFfmpeg(ctx)
	case ToolTranscription:
		return s.probeTranscription(ctx)
	case ToolChrome:
		return s.probeChrome(ctx)
	case ToolCodex:
		return s.probeCodex(ctx)
	case ToolCodexLogin:
		return s.probeChatGPT(ctx)
	}
	return Row{ID: id, State: StateFailed, Hint: "Không nhận diện được công cụ"}
}

// applyUpdateInfo đổi hàng ready thành update khi kiểm cập nhật thấy bản mới.
func (s *Service) applyUpdateInfo(row Row, latest string) Row {
	if row.State != StateReady || row.Version == "" || latest == "" {
		return row
	}
	if tools.CompareVersions(row.Version, latest) >= 0 {
		return row
	}
	row.State = StateUpdate
	row.Update = &UpdateInfo{Current: row.Version, Latest: latest}
	row.Hint = fmt.Sprintf("Có bản %s · bạn đang dùng %s. YouTube hay đổi cách tải, nên cập nhật.", latest, row.Version)
	return row
}

// checkLatest hỏi GitHub về release yt-dlp mới nhất theo nhịp updateCheckTTL;
// lỗi mạng được giữ để không hỏi lại liên tục.
func (s *Service) checkLatest(ctx context.Context, id ToolID) {
	s.mu.Lock()
	prev := s.latest[id]
	s.mu.Unlock()
	if s.now().Sub(prev.at) < updateCheckTTL {
		return
	}
	latest, err := s.tools.YtDlp.LatestVersion(ctx)
	s.mu.Lock()
	s.latest[id] = latestCheck{latest: latest, at: s.now(), err: err}
	s.mu.Unlock()
	if err == nil && latest != "" {
		s.emit(id)
	}
}

func (s *Service) probeYtDlp(ctx context.Context) Row {
	version, path, installed := s.tools.YtDlp.Status(ctx)
	go s.checkLatest(ctx, ToolYtDlp) // nhịp 6h: một request HEAD tới GitHub
	if !installed {
		return s.installHint(ToolYtDlp, Row{
			ID: ToolYtDlp, State: StateMissing,
			Hint: "Chưa cài — app sẽ tự cài bản ghim.",
		})
	}
	row := Row{
		ID: ToolYtDlp, State: StateReady, Version: version,
		Hint: fmt.Sprintf("Lấy video, thông tin và lời thoại từ YouTube · bản %s", version),
	}
	if path != "" {
		row.Detail = path
	}
	return row
}

func (s *Service) probeFfmpeg(ctx context.Context) Row {
	st := s.tools.Ffmpeg.Status(ctx)
	switch st.State {
	case tools.FfmpegReady:
		return Row{
			ID: ToolFfmpeg, State: StateReady, Version: st.Version,
			Hint:   fmt.Sprintf("Cắt cảnh, xử lý âm thanh và dựng video · bản %s", st.Version),
			Detail: st.Path,
		}
	case tools.FfmpegMissing:
		return s.installHint(ToolFfmpeg, Row{
			ID: ToolFfmpeg, State: StateMissing,
			Hint: "Chưa cài — app sẽ tự cài bản ghim.",
		})
	default:
		hint := "Bản ffmpeg dùng được nhưng thiếu thành phần — app sẽ cài bản ghim."
		if st.Error != "" {
			hint = friendlyInstallError(st.Error)
		}
		if len(st.Missing) > 0 {
			hint = "Bản ffmpeg thiếu " + strings.Join(st.Missing, ", ") + " — app sẽ cài bản ghim."
		}
		return Row{
			ID: ToolFfmpeg, State: StateFailed,
			Hint: hint, Detail: firstNonEmpty(st.Error, st.Path),
		}
	}
}

func (s *Service) probeTranscription(ctx context.Context) Row {
	st := s.trans.Status(ctx)
	switch st.State {
	case tools.TranscriptionReady:
		return Row{
			ID: ToolTranscription, State: StateReady, Version: st.FasterWhisper,
			Hint:   "Chép lời thoại thành chữ ngay trên máy · mô hình giọng nói đã tải",
			Detail: st.EnvDir,
		}
	case tools.TranscriptionBroken:
		return s.installHint(ToolTranscription, Row{
			ID: ToolTranscription, State: StateFailed,
			Hint:   "Môi trường tách tiếng bị hỏng — cài lại sẽ sửa.",
			Detail: st.Error,
		})
	default:
		return s.installHint(ToolTranscription, Row{
			ID: ToolTranscription, State: StateMissing,
			Hint: "Chưa cài — app sẽ tự cài bản ghim.",
		})
	}
}

func (s *Service) probeChrome(ctx context.Context) Row {
	// Bản máy chủ không bao giờ mở Chrome của chủ máy (xem tools.Chrome.Resolve),
	// nên câu chữ không được hứa “Chrome trên máy”: nói rõ máy chủ dùng bản
	// Chrome for Testing riêng của nó.
	hosted := uploadlimit.Hosted()
	st := s.tools.Chrome.Status(ctx)
	if st.State != tools.ChromeReady {
		if st.State == tools.ChromeMissing {
			hint := "Chưa cài — app sẽ tự cài Chrome for Testing."
			if hosted {
				hint = "Chưa cài — máy chủ sẽ tự cài Chrome for Testing riêng, không dùng Chrome của máy."
			}
			return s.installHint(ToolChrome, Row{
				ID: ToolChrome, State: StateMissing,
				Hint: hint,
			})
		}
		hint := "Chrome trên máy không dùng được cho xuất video — app sẽ cài Chrome for Testing."
		if hosted {
			hint = "Chrome for Testing chưa dùng được cho xuất video — app sẽ cài lại bản ghim."
		}
		return Row{
			ID: ToolChrome, State: StateFailed,
			Hint:   hint,
			Detail: st.Error,
		}
	}
	caps := s.tools.Chrome.ExportCaps(ctx)
	if caps.State != tools.ExportReady {
		hint := "Chrome chạy được nhưng không xuất được video — app sẽ cài Chrome for Testing."
		if hosted {
			hint = "Chrome for Testing chạy được nhưng không xuất được video trên máy chủ — app sẽ cài lại bản ghim."
		}
		return s.installHint(ToolChrome, Row{
			ID: ToolChrome, State: StateFailed,
			Hint:    hint,
			Detail:  caps.Error,
			Version: st.Version,
		})
	}
	provided := st.Source != "" && st.Source != tools.ChromeManaged
	hint := fmt.Sprintf("Dùng Chrome chạy ngầm để xuất video và đo âm thanh · bản %s", st.Version)
	switch {
	case hosted && !provided:
		hint = fmt.Sprintf("Máy chủ dùng Chrome for Testing riêng, không dùng Chrome của máy · bản %s", st.Version)
	case hosted:
		hint = fmt.Sprintf("Dùng Chrome bạn chọn, chạy ngầm để xuất video và đo âm thanh · bản %s", st.Version)
	case provided:
		hint = fmt.Sprintf("Dùng Chrome có sẵn trên máy, chạy ngầm để xuất video và đo âm thanh · bản %s", st.Version)
	}
	return Row{
		ID: ToolChrome, State: StateReady, Version: st.Version,
		Hint:     hint,
		Detail:   st.Path,
		Provided: provided,
	}
}

func (s *Service) probeCodex(ctx context.Context) Row {
	st := s.tools.Codex.Status(ctx)
	switch st.State {
	case tools.CodexReady, tools.CodexLoggedOut, tools.CodexIsolationUnverified:
		version := ""
		path := ""
		// Codex app không tự tải (có sẵn trên máy hoặc người dùng chọn) là “Có
		// sẵn”, như Chrome: không được ghi “Đã cài · <dung lượng bản ghim>”.
		provided := false
		if st.Binary != nil {
			version = st.Binary.Version
			path = st.Binary.Path
			provided = st.Binary.Source != tools.SourceManaged
		}
		hint := fmt.Sprintf("Vẽ ảnh cho từng cảnh · chạy tách riêng, không đụng dữ liệu khác · bản %s", version)
		detail := path
		if st.State == tools.CodexIsolationUnverified {
			detail = "cách ly chưa kiểm chứng: " + path
		}
		return Row{ID: ToolCodex, State: StateReady, Version: version, Hint: hint, Detail: detail, Provided: provided}
	case tools.CodexMissing:
		return s.installHint(ToolCodex, Row{
			ID: ToolCodex, State: StateMissing,
			Hint: "Chưa cài — app sẽ tự cài bản ghim.",
		})
	case tools.CodexOutdated:
		return s.installHint(ToolCodex, Row{
			ID: ToolCodex, State: StateFailed,
			Hint: fmt.Sprintf("Bản codex quá cũ — app cần tối thiểu %s, sẽ cài bản ghim.", st.MinVersion),
		})
	default:
		return s.installHint(ToolCodex, Row{
			ID: ToolCodex, State: StateFailed,
			Hint: "Codex trên máy không chạy được — app sẽ cài bản ghim.",
		})
	}
}

func (s *Service) probeChatGPT(ctx context.Context) Row {
	login := ""
	if s.chatgpt != nil {
		if v, err := s.chatgpt.ChatGPTLogin(ctx); err == nil {
			login = v
		}
	} else {
		login = string(s.tools.Codex.Status(ctx).Login)
	}
	if login == string(tools.CodexLoginChatGPT) || login == string(tools.CodexLoginAPIKey) {
		return Row{
			ID: ToolCodexLogin, State: StateReady,
			Hint: "Đã đăng nhập · quản lý ở Kết nối AI",
		}
	}
	return Row{
		ID: ToolCodexLogin, State: StateNeedsLogin,
		Hint: "Ảnh được vẽ bằng tài khoản ChatGPT của bạn, nên bước này cần bạn tự đăng nhập.",
	}
}

// installHint phủ lỗi của lượt cài gần nhất lên hàng: cài xong không đạt
// hay bị lỗi giữa chừng vẫn giữ nguyên câu lỗi thay vì "Chưa cài" trơ trẽn.
func (s *Service) installHint(id ToolID, row Row) Row {
	if s.mgr == nil {
		return row
	}
	st := s.mgrState(id)
	if st == nil {
		return row
	}
	if st.Phase == toolchain.PhaseFailed && st.Error != nil && *st.Error != "" {
		row.Hint = friendlyInstallError(*st.Error)
		row.Detail = *st.Error
		if row.State != StateInstalling {
			row.State = StateFailed
		}
	}
	if st.Phase == toolchain.PhaseCancelled {
		row.Hint = "Đã huỷ giữa chừng — bấm Cài lại để tiếp tục."
		if row.State != StateInstalling {
			row.State = StateFailed
		}
	}
	return row
}

// mgrState đọc install-state của một công cụ (yt-dlp không có — nil).
func (s *Service) mgrState(id ToolID) *toolchain.InstallState {
	mid, ok := managedID(id)
	if !ok {
		return nil
	}
	st, err := s.mgr.State(mid)
	if err != nil {
		return nil
	}
	return st
}

// liveProgress đọc tiến độ trực tiếp của lượt cài cho thẻ/pump.
func liveProgress(mgr *tools.Managed, id ToolID) *domain.InstallProgress {
	mid, ok := managedID(id)
	if !ok || mgr == nil {
		return nil
	}
	st, err := mgr.State(mid)
	if err != nil || st == nil {
		return nil
	}
	switch st.Phase {
	case toolchain.PhaseDone, toolchain.PhaseFailed, toolchain.PhaseCancelled:
		return nil
	}
	p := &domain.InstallProgress{
		Phase:      string(st.Phase),
		BytesDone:  st.BytesDone,
		BytesTotal: st.BytesTotal,
		StartedAt:  st.StartedAt.Unix(),
	}
	if st.Detail != nil {
		p.Detail = *st.Detail
	}
	return p
}

// managedID ánh hàng cài đặt sang id của sổ công cụ app-managed
// (yt-dlp và hàng đăng nhập nằm ngoài sổ).
func managedID(id ToolID) (tools.ManagedToolID, bool) {
	switch id {
	case ToolFfmpeg:
		return tools.ManagedFfmpeg, true
	case ToolTranscription:
		return tools.ManagedTranscription, true
	case ToolChrome:
		return tools.ManagedChrome, true
	case ToolCodex:
		return tools.ManagedCodex, true
	}
	return "", false
}

// friendlyInstallError đổi lỗi cài thành câu ngắn cho người dùng; lỗi lạ
// giữ nguyên bản gốc cho Chi tiết.
func friendlyInstallError(raw string) string {
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "checksum"):
		return "Tệp tải về lệch chữ ký — app sẽ tải lại khi bạn bấm Tải tiếp."
	case strings.Contains(lower, "cancelled") || strings.Contains(lower, "canceled") || strings.Contains(lower, "huỷ") || strings.Contains(lower, "hủy"):
		return "Đã huỷ giữa chừng — bấm Tải tiếp để cài tiếp từ chỗ dừng."
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline"):
		return "Tải quá chậm nên bị ngắt — bấm Tải tiếp để cài tiếp từ chỗ dừng."
	case strings.Contains(lower, "network") || strings.Contains(lower, "connection") ||
		strings.Contains(lower, "dns") || strings.Contains(lower, "eof") ||
		strings.Contains(lower, "unreachable") || strings.Contains(lower, "mạng"):
		return "Mạng bị ngắt giữa chừng — bấm Tải tiếp để cài tiếp từ chỗ dừng."
	case strings.Contains(lower, "no space") || strings.Contains(lower, "không đủ chỗ") || strings.Contains(lower, "disk"):
		return "Ổ đĩa hết chỗ — dọn thêm chỗ trống rồi bấm Tải tiếp."
	}
	if len(raw) > 200 {
		return raw[:200]
	}
	return raw
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
