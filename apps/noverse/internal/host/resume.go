package host

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/revision"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
)

func upgradeProject(st *storepkg.Store) error {
	version, err := st.LoadProjectFormatVersion()
	if err != nil {
		return fmt.Errorf("đọc phiên bản định dạng dự án thất bại: %w", err)
	}
	if version > storepkg.CurrentProjectFormatVersion {
		return fmt.Errorf("phiên bản định dạng dự án v%d cao hơn v%d mà chương trình hiện tại hỗ trợ, hãy nâng cấp ainovel-cli", version, storepkg.CurrentProjectFormatVersion)
	}
	for version < storepkg.CurrentProjectFormatVersion {
		next := version + 1
		switch version {
		case storepkg.LegacyProjectFormatVersion:
			if err := migrateLegacyBook(st); err != nil {
				return fmt.Errorf("nâng cấp dữ liệu dự án v%d→v%d thất bại: %w", version, next, err)
			}
			if err := revision.MigrateLegacyBaseline(st); err != nil {
				return fmt.Errorf("nâng cấp dữ liệu dự án v%d→v%d thất bại: %w", version, next, err)
			}
		default:
			return fmt.Errorf("không hỗ trợ nâng cấp từ định dạng dự án v%d", version)
		}
		if err := st.SaveProjectFormatVersion(next); err != nil {
			return fmt.Errorf("lưu phiên bản định dạng dự án v%d thất bại: %w", next, err)
		}
		slog.Info("Nâng cấp dữ liệu dự án hoàn tất", "module", "migration", "from", version, "to", next)
		version = next
	}
	return nil
}

func migrateLegacyBook(st *storepkg.Store) error {
	book, err := st.Book.Load()
	if err != nil {
		return err
	}
	if book == nil {
		book, err = loadLegacyBook(st)
		if err != nil || book == nil {
			return err
		}
	}
	if err := st.Book.Save(*book); err != nil {
		return fmt.Errorf("lưu thông tin tác phẩm cũ thất bại: %w", err)
	}
	if _, err := st.Checkpoints.AppendArtifact(domain.GlobalScope(), "book", "meta/book.json"); err != nil {
		return fmt.Errorf("ghi lại thông tin tác phẩm cũ thất bại: %w", err)
	}
	return nil
}

func loadLegacyBook(st *storepkg.Store) (*domain.BookMetadata, error) {
	dir, fileBacked := st.FileRoot()
	if !fileBacked {
		return nil, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "meta", "progress.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("đọc tiến độ tác phẩm cũ thất bại: %w", err)
	}
	var legacy struct {
		NovelName string `json:"novel_name"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("phân tích tiến độ tác phẩm cũ thất bại: %w", err)
	}
	legacy.NovelName = strings.TrimSpace(legacy.NovelName)
	if legacy.NovelName == "" {
		return nil, nil
	}
	premise, err := st.Outline.LoadPremise()
	if err != nil {
		return nil, fmt.Errorf("đọc tiền đề truyện cũ thất bại: %w", err)
	}
	title := legacyPremiseTitle(premise)
	if title == "" {
		return nil, fmt.Errorf("tiền đề truyện cũ thiếu tiêu đề tên tác phẩm")
	}
	if title != legacy.NovelName {
		return nil, fmt.Errorf("xung đột tên tác phẩm cũ: progress=%q, premise=%q", legacy.NovelName, title)
	}
	// 旧 premise 多为中文标题；越南语项目的 premise 用 "Xung đột cốt lõi"（与 tools.premiseHeadingAliases 一致）。
	synopsis := legacyPremiseSection(premise, "核心冲突", "Xung đột cốt lõi")
	if synopsis == "" {
		return nil, fmt.Errorf("tiền đề truyện cũ thiếu mục «Xung đột cốt lõi», không tạo được giới thiệu tác phẩm")
	}
	return &domain.BookMetadata{Title: title, Synopsis: synopsis}, nil
}

func legacyPremiseTitle(premise string) string {
	for _, line := range strings.Split(premise, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "# ")), "《》«»\"")
		}
	}
	return ""
}

func legacyPremiseSection(premise string, headings ...string) string {
	var body []string
	matched := false
	for _, line := range strings.Split(premise, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			if matched {
				break
			}
			matched = slices.Contains(headings, strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")))
			continue
		}
		if matched {
			body = append(body, line)
		}
	}
	return strings.TrimSpace(strings.Join(body, "\n"))
}

// resumeLabel 基于事实生成 Resume 的 UI 标签。
// label 为空表示无可恢复状态（应走新建）。恢复本身不需要任何 prompt——
// Engine 只恢复事实：从 store 重算路由续跑（docs/engine-rfc.md §6）。
func resumeLabel(store *storepkg.Store) (string, error) {
	progress, err := store.Progress.Load()
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if progress == nil || progress.Phase == domain.PhaseComplete {
		return "", nil
	}
	return describeResume(store, progress)
}

// describeResume 生成人类可读的恢复标签；不影响 Engine 路由。
// 所有执行路由由 Flow Router 按事实推导；这里仅面向 UI 的 "恢复：xxx"。
func describeResume(store *storepkg.Store, progress *domain.Progress) (string, error) {
	switch progress.Phase {
	case domain.PhaseInit, domain.PhasePremise, domain.PhaseOutline:
		return describePlanningResume(store)
	case domain.PhaseWriting:
		// 优先级与 Router 的决策优先级对齐，让 label 与即将派发的指令一致。
		pending, err := store.Signals.LoadPendingCommit()
		if err != nil {
			return "", fmt.Errorf("đọc bản nộp chờ khôi phục thất bại: %w", err)
		}
		if pending != nil {
			return fmt.Sprintf("Khôi phục: nộp chương %d bị gián đoạn", pending.Chapter), nil
		}
		if len(progress.PendingRewrites) > 0 {
			verb := "viết lại"
			if progress.Flow == domain.FlowPolishing {
				verb = "gọt giũa"
			}
			return fmt.Sprintf("Khôi phục %s: %d chương chờ xử lý", verb, len(progress.PendingRewrites)), nil
		}
		if progress.Flow == domain.FlowReviewing {
			return "Khôi phục: thẩm duyệt bị gián đoạn", nil
		}
		if progress.InProgressChapter > 0 {
			return fmt.Sprintf("Khôi phục: chương %d đang viết", progress.InProgressChapter), nil
		}
		label, err := describeArcEndLabel(store, progress)
		if err != nil {
			return "", err
		}
		if label != "" {
			return label, nil
		}
		return fmt.Sprintf("Khôi phục: tiếp tục từ chương %d", progress.NextChapter()), nil
	}
	return "Khôi phục", nil
}

// describePlanningResume 生成规划期的恢复标签：写下一个将跑的步骤，而不是阶段名
// （设计 21-dev-prompt.md Gói O mục 1：“Khôi phục · tiếp tục từ Lưu dàn ý”）。
// 推导与 Flow Router 同事实：Route 的规划分支按 FoundationMissing 派同一规划师
// 补齐缺项（tier 未落盘时 planStartFallback 也按 PlanStartRecord 的同一任务续跑），
// 所以下一个要落的工件 = 缺项表的第一项，文案走 07-copy.md › Bảng từ 的保存动作名。
// 缺项为空（理论上只在阶段滞后于事实时出现）或键未登记 → 退回泛化 “Khôi phục”。
func describePlanningResume(store *storepkg.Store) (string, error) {
	missing, err := store.FoundationMissing()
	if err != nil {
		return "", fmt.Errorf("đọc trạng thái thiết lập nền: %w", err)
	}
	if len(missing) == 0 {
		return "Khôi phục", nil
	}
	step := FoundationStepLabel(missing[0])
	if step == "" {
		return "Khôi phục", nil
	}
	return fmt.Sprintf("Khôi phục · tiếp tục từ %s", step), nil
}

// describeArcEndLabel 为弧末/卷末的多种中间状态生成贴合 UI 的标签。
// 与 flow.Route 的弧末分支保持同序，保证 label 与 Router 首条指令对齐。
func describeArcEndLabel(store *storepkg.Store, progress *domain.Progress) (string, error) {
	if !progress.Layered || len(progress.CompletedChapters) == 0 {
		return "", nil
	}
	lastCh := progress.CompletedChapters[len(progress.CompletedChapters)-1]
	boundary, err := store.Outline.CheckArcBoundary(lastCh)
	if err != nil {
		return "", fmt.Errorf("Kiểm tra ranh giới cung: %w", err)
	}
	if boundary == nil || !boundary.IsArcEnd {
		return "", nil
	}
	vol, arc := boundary.Volume, boundary.Arc
	hasArcReview, err := store.World.HasArcReview(lastCh)
	if err != nil {
		return "", fmt.Errorf("Đọc thẩm duyệt cung: %w", err)
	}
	hasArcSummary, err := store.Summaries.HasArcSummary(vol, arc)
	if err != nil {
		return "", fmt.Errorf("Đọc tóm tắt cung: %w", err)
	}
	hasVolumeSummary := false
	if boundary.IsVolumeEnd {
		hasVolumeSummary, err = store.Summaries.HasVolumeSummary(vol)
		if err != nil {
			return "", fmt.Errorf("Đọc tóm tắt tập: %w", err)
		}
	}
	switch {
	case !hasArcReview:
		return fmt.Sprintf("Khôi phục: thẩm duyệt cuối cung đang chờ (Tập %d · Cung %d)", vol, arc), nil
	case !hasArcSummary:
		return fmt.Sprintf("Khôi phục: tóm tắt cung chờ tạo (Tập %d · Cung %d)", vol, arc), nil
	case boundary.IsVolumeEnd && !hasVolumeSummary:
		return fmt.Sprintf("Khôi phục: tóm tắt tập chờ tạo (Tập %d)", vol), nil
	case boundary.NeedsExpansion && boundary.NextArc > 0:
		return fmt.Sprintf("Khôi phục: chờ triển khai cung kế tiếp (Tập %d · Cung %d)", boundary.NextVolume, boundary.NextArc), nil
	case boundary.NeedsNewVolume:
		return fmt.Sprintf("Khôi phục: chờ quyết định tập kế tiếp (cuối Tập %d)", vol), nil
	}
	return "", nil
}
