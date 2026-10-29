package tools

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/agentcore/schema"
	"github.com/datnt1998/verse/core/llmcontract"
)

// SaveFoundationTool 保存基础设定（premise/outline/characters），Architect 专用。
type SaveFoundationTool struct {
	store       *store.Store
	pack        RetentionPack
	ledgerModel llmtypes.ChatModel
}

func NewSaveFoundationTool(store *store.Store, packs ...RetentionPack) *SaveFoundationTool {
	t := &SaveFoundationTool{store: store}
	if len(packs) > 0 {
		t.pack = packs[0]
	}
	return t
}

func (t *SaveFoundationTool) WithLedgerModel(model llmtypes.ChatModel) *SaveFoundationTool {
	t.ledgerModel = model
	return t
}

func (t *SaveFoundationTool) Name() string { return "save_foundation" }
func (t *SaveFoundationTool) Description() string {
	return "Lưu thiết lập nền tảng của truyện (premise/outline/characters/world_rules/compass...). **Đây là cửa vào lưu trữ duy nhất**: nội dung chưa qua công cụ này sẽ không vào store, chỉ xuất Markdown/JSON trong tin nhắn nghĩa là mất. Tham số cố định là {type, content, scale?, volume?, arc?}. type chọn một trong premise / outline / layered_outline / characters / world_rules / expand_arc / append_volume / update_compass / complete_book. Với premise, content phải là chuỗi Markdown; các type khác content ưu tiên truyền thẳng mảng JSON hoặc object. expand_arc hiệu chỉnh và mở rộng một cung khung chưa viết (số chương chi tiết của một cung không được vượt 8 chương — thẩm duyệt cuối cung cần đọc trọn cung một lần, quá dài không thẩm duyệt được; vượt hạn mức hãy tách thành nhiều cung, cần volume + arc, content là {title, goal, chapters}, có thể sửa mục tiêu khung gốc theo nội dung đã hoàn thành); append_volume thêm tập mới (content là JSON VolumeOutline đầy đủ, gồm cấu trúc cung; tầng trên cùng có \"final\": true là tuyên bố tập kết thúc — toàn sách khép lại ở tập đó, viết xong mọi chương sẽ tự hoàn tất, không cần gọi complete_book nữa); update_compass cập nhật hướng kết cục (content là JSON StoryCompass, trường chỉ gồm {ending_direction: string bắt buộc, open_threads?: string[], estimated_scale?: string}, mọi trường khác đều không được nhận); complete_book tuyên bố toàn sách hoàn tất (content truyền object rỗng {}, đẩy thẳng Phase=Complete; công cụ sẽ kiểm tra: mọi chương trong đại cương đã viết xong, không có hàng đợi viết lại, compass không còn open_threads chưa khép — muốn xác nhận tuyến dài đã khép phải update_compass làm rỗng open_threads xuống đĩa trước, muốn kết thúc sớm dùng tập kết thúc final của append_volume). append_volume / complete_book bắt buộc có tham số reason (một câu lý do phán định, đối chiếu danh mục phán định hoàn tất, ghi vào nhật ký phán định). scale tùy chọn, chỉ cho phép short / mid / long."
}
func (t *SaveFoundationTool) Label() string { return "Lưu thiết lập" }

// 写工具（跨域更新 Outline/Progress/Characters），禁止并发。
func (t *SaveFoundationTool) ReadOnly(_ json.RawMessage) bool        { return false }
func (t *SaveFoundationTool) ConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *SaveFoundationTool) Schema() map[string]any {
	return schema.Object(
		schema.Property("type", schema.Enum("Loại thiết lập", "premise", "outline", "layered_outline", "characters", "world_rules", "expand_arc", "append_volume", "update_compass", "complete_book")).Required(),
		schema.Property("content", map[string]any{
			"description": "Nội dung. premise truyền chuỗi Markdown; các type khác truyền thẳng mảng JSON hoặc object, cũng chấp nhận chuỗi JSON. Với expand_arc truyền {title, goal, chapters}. Mỗi chương outline/layered_outline/expand_arc/append_volume nhận thêm open_questions (mảng chuỗi), promise (chuỗi; tiền tố da_tra: khi cảnh tiêu đề được diễn trọn), hook_type (chỉ lat_thong_tin/de_doa_moi/quyet_dinh_treo/cau_thoai_no; chương cuối có thể seed), midpoint_twist (boolean, false nếu không có cú lật). Khi gói khai giu_chan.bat_buoc, phải điền đủ các trường đó. Với characters, mỗi mục là {name, role, kind, revealAt, description, arc, traits}: kind bắt buộc, một trong lead/antagonist/ally/support (chính/phản diện-đồng phạm/đồng minh-bạn/phụ); revealAt là số chương lộ vai thật khi nhân vật có bí mật (null hoặc bỏ trống nếu không).",
		}).Required(),
		schema.Property("scale", schema.Enum("Tầng quy hoạch", "short", "mid", "long")),
		schema.Property("volume", schema.Int("Số thứ tự tập mục tiêu, tính từ 1 (chỉ bắt buộc khi expand_arc)")),
		schema.Property("arc", schema.Int("Số thứ tự cung mục tiêu trong tập, tính từ 1 (chỉ bắt buộc khi expand_arc)")),
		schema.Property("reason", schema.String("Lý do phán định cuối tập (bắt buộc khi append_volume / complete_book): đối chiếu danh mục phán định hoàn tất, một câu nói rõ vì sao nối tập, tuyên bố tập kết thúc hay hoàn tất")),
		schema.Property("nguoi_nghe", schema.Array("Luôn lưu cùng premise, kể cả không dùng gói: chọn nhóm phù hợp với yêu cầu ban đầu, không hỏi lại. Chỉ số nhóm Master §1.2: 0 Phụ nữ trung niên nghe khi làm việc nhà, 1 Nữ 18–35, 2 Nam trung niên, 3 Nam trẻ, 4 Người cao tuổi. Mặc định lấy từ gói khi gói khai một nhóm; gói C chọn [2] (C-Nam) hoặc [1] (C-Nữ).", map[string]any{"type": "integer", "enum": []any{0, 1, 2, 3, 4}})),
		schema.Property("diem_khac", schema.String("Lưu cùng premise: Điểm khác theo Master §17, so với tên và tiền đề các truyện trước trong ngữ cảnh. Chỉ viết câu tiếng Việt cho người đọc; cấm ghi khóa nội bộ, tên trường hoặc giải thích thiếu ngữ cảnh. Không có truyện trước để đối chiếu thì bỏ qua hoặc để trống.")),
		schema.Property("hat_giong_phan_hai", schema.String("Lưu cùng premise: câu hỏi lớn mở phần hai mà không phá kết truyện này (Master §19, cảnh báo nếu thiếu).")),
		schema.Property("address_terms", nullableAddressTermsSchema()),
		schema.Property("axes", llmcontract.Nullable(schema.Array("Lưu cùng premise: đúng mười giá trị theo thứ tự Bộ quy tắc và biến thể; Bối cảnh; Nghề của nhân vật chính; Kiểu phản diện; Điểm thương; Vũ khí và cơ chế lật; Sân khấu cao trào; Tông giọng; Cấu trúc mở đầu; Kiểu kết. Không dùng với thiết lập khác thì null.", schema.String("Giá trị trục")))).Required(),
	)
}

func (t *SaveFoundationTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Type         string                    `json:"type"`
		Content      json.RawMessage           `json:"content"`
		Scale        string                    `json:"scale"`
		Volume       int                       `json:"volume"`
		Arc          int                       `json:"arc"`
		Reason       string                    `json:"reason"`
		AddressTerms []domain.AddressTermInput `json:"address_terms"`
		Axes         []string                  `json:"axes"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid args: %w: %w", errs.ErrToolArgs, err)
	}
	if len(a.AddressTerms) > 0 && a.Type != "characters" {
		return nil, fmt.Errorf("address_terms chỉ dùng với type=characters, hiệu lực từ chương 1: %w", errs.ErrToolArgs)
	}
	content, err := normalizeFoundationContent(a.Content)
	if err != nil {
		return nil, err
	}
	if a.Scale != "" {
		switch domain.PlanningTier(a.Scale) {
		case domain.PlanningTierShort, domain.PlanningTierMid, domain.PlanningTierLong:
		default:
			return nil, fmt.Errorf("invalid scale %q, expected short/mid/long: %w", a.Scale, errs.ErrToolArgs)
		}
	}

	result := map[string]any{"saved": true, "type": a.Type, "scale": a.Scale}

	// 全量大纲只属于规划期。写作期必须用受保护的增量操作，完结后必须先重开；
	// 否则会绕过已完成章节保护，破坏 Progress 与章节事实的一致性。
	progress, err := t.store.Progress.Load()
	if err != nil {
		return nil, fmt.Errorf("check foundation phase: %w: %w", errs.ErrStoreRead, err)
	}
	if (a.Type == "outline" || a.Type == "layered_outline") && progress != nil {
		switch progress.Phase {
		case domain.PhaseWriting:
			return nil, fmt.Errorf(
				"giai đoạn viết không được dùng %s ghi đè toàn bộ đại cương. Hãy dùng revise_outline sửa các chương chưa diễn ra, expand_arc mở rộng cung khung, hoặc append_volume thêm tập mới: %w",
				a.Type, errs.ErrToolPrecondition)
		case domain.PhaseComplete:
			return nil, fmt.Errorf(
				"sách đã hoàn tất, không được dùng %s ghi đè toàn bộ đại cương. Hãy mở lại tác phẩm trước, rồi dùng thao tác tu chỉnh đại cương hoặc viết tiếp có bảo vệ: %w",
				a.Type, errs.ErrToolPrecondition)
		}
	}
	if a.Type == "premise" || a.Type == "outline" || a.Type == "layered_outline" {
		block, err := t.checkLedger(ctx, a.Type, content, a.Axes)
		if err != nil {
			return nil, err
		}
		if block != nil {
			return json.Marshal(block)
		}
	}
	if a.Scale != "" {
		if err := t.store.RunMeta.SetPlanningTier(domain.PlanningTier(a.Scale)); err != nil {
			return nil, fmt.Errorf("save planning tier: %w: %w", errs.ErrStoreWrite, err)
		}
	}

	// 卷末三选一（续卷/收官/完结）是全书最重的语义判断，理由必须成为审计事实
	// （decisions.jsonl，与 plan_start/intervention 同一条流水），否则收官过早/
	// 续卷失当只能翻会话日志排障。事实快照取判定时刻（变更落盘前）的进度。
	volumeEnd := a.Type == "append_volume" || a.Type == "complete_book"
	if volumeEnd && strings.TrimSpace(a.Reason) == "" {
		return nil, fmt.Errorf("%s bắt buộc có tham số reason: đối chiếu danh mục phán định hoàn tất, một câu nói rõ lần này vì sao nối tập, tuyên bố tập kết thúc hay hoàn tất: %w", a.Type, errs.ErrToolArgs)
	}
	var volumeEndFacts json.RawMessage
	if volumeEnd {
		p, err := t.store.Progress.Load()
		if err != nil {
			return nil, fmt.Errorf("load progress for volume-end facts: %w: %w", errs.ErrStoreRead, err)
		}
		if p != nil {
			facts := map[string]any{"completed_chapters": len(p.CompletedChapters)}
			if p.Layered {
				outline, outlineErr := t.store.Outline.LoadOutline()
				if outlineErr != nil {
					return nil, fmt.Errorf("load outlined chapters for volume-end facts: %w: %w", errs.ErrStoreRead, outlineErr)
				}
				facts["dynamic_planning"] = true
				facts["outlined_chapters"] = len(outline)
			} else {
				facts["total_chapters"] = p.TotalChapters
			}
			volumeEndFacts, err = json.Marshal(facts)
			if err != nil {
				return nil, fmt.Errorf("marshal volume-end facts: %w", err)
			}
		}
	}

	decode := func(typeName string, out any) error {
		return decodeFoundationJSON(typeName, content, out)
	}

	switch a.Type {
	case "premise":
		fields, err := t.store.Outline.LoadStoryRetention()
		if err != nil {
			return nil, fmt.Errorf("load story retention: %w: %w", errs.ErrStoreRead, err)
		}
		if err := json.Unmarshal(args, &fields); err != nil {
			return nil, fmt.Errorf("decode story retention: %w: %w", errs.ErrToolArgs, err)
		}
		others, err := siblingStories(t.store)
		if err != nil {
			return nil, fmt.Errorf("load comparison stories: %w: %w", errs.ErrStoreRead, err)
		}
		if len(others) == 0 {
			fields.DiemKhac = ""
		}
		fields, err = normalizeStoryRetention(fields, t.pack)
		if err != nil {
			return nil, err
		}
		if err := t.store.Outline.SaveStoryRetention(fields); err != nil {
			return nil, fmt.Errorf("save story retention: %w: %w", errs.ErrStoreWrite, err)
		}
		result["story_retention"] = fields
		if err := t.store.Outline.SavePremise(content); err != nil {
			return nil, fmt.Errorf("save premise: %w: %w", errs.ErrStoreWrite, err)
		}
		if err := t.store.Progress.AdvancePhase(domain.PhasePremise); err != nil {
			return nil, fmt.Errorf("update premise phase: %w: %w", errs.ErrStoreWrite, err)
		}

	case "outline":
		var entries []domain.OutlineEntry
		if err := decode("outline", &entries); err != nil {
			return nil, err
		}
		if err := validateRetention(t.store, t.pack, json.RawMessage(content), true); err != nil {
			return nil, err
		}
		if defect := domain.StalledOutline(entries); defect != "" {
			return nil, fmt.Errorf("%s: %w", defect, errs.ErrToolArgs)
		}
		if err := t.store.Outline.SaveOutline(entries); err != nil {
			return nil, fmt.Errorf("save outline: %w: %w", errs.ErrStoreWrite, err)
		}
		if err := t.store.Progress.AdvancePhase(domain.PhaseOutline); err != nil {
			return nil, fmt.Errorf("update outline phase: %w: %w", errs.ErrStoreWrite, err)
		}
		if err := t.store.Progress.SetTotalChapters(len(entries)); err != nil {
			return nil, fmt.Errorf("set total chapters: %w: %w", errs.ErrStoreWrite, err)
		}
		if domain.PlanningTier(a.Scale) != domain.PlanningTierLong {
			if err := t.store.Progress.SetLayered(false); err != nil {
				return nil, fmt.Errorf("disable layered mode: %w: %w", errs.ErrStoreWrite, err)
			}
			if err := t.store.Progress.UpdateVolumeArc(0, 0); err != nil {
				return nil, fmt.Errorf("reset volume/arc: %w: %w", errs.ErrStoreWrite, err)
			}
			if err := t.store.Outline.ClearLayeredOutline(); err != nil {
				return nil, fmt.Errorf("clear layered outline: %w: %w", errs.ErrStoreWrite, err)
			}
		}
		result["chapters"] = len(entries)

	case "layered_outline":
		var volumes []domain.VolumeOutline
		if err := decode("layered_outline", &volumes); err != nil {
			return nil, err
		}
		for vi := range volumes {
			for ai := range volumes[vi].Arcs {
				arc := &volumes[vi].Arcs[ai]
				if defect := domain.OversizedArc(
					fmt.Sprintf("tập %d cung %d", volumes[vi].Index, arc.Index), arcPlannedSize(arc)); defect != "" {
					return nil, fmt.Errorf("%s: %w", defect, errs.ErrToolArgs)
				}
			}
		}
		if defect := domain.StalledOutline(domain.FlattenOutline(volumes)); defect != "" {
			return nil, fmt.Errorf("%s: %w", defect, errs.ErrToolArgs)
		}
		if err := validateRetention(t.store, t.pack, json.RawMessage(content), false); err != nil {
			return nil, err
		}
		if err := t.store.Outline.SaveLayeredOutline(volumes); err != nil {
			return nil, fmt.Errorf("save layered_outline: %w: %w", errs.ErrStoreWrite, err)
		}
		total := domain.EstimatedChapterCapacity(volumes)
		if err := t.store.Progress.AdvancePhase(domain.PhaseOutline); err != nil {
			return nil, fmt.Errorf("update outline phase: %w: %w", errs.ErrStoreWrite, err)
		}
		if err := t.store.Progress.SetTotalChapters(total); err != nil {
			return nil, fmt.Errorf("set total chapters: %w: %w", errs.ErrStoreWrite, err)
		}
		if err := t.store.Progress.SetLayered(true); err != nil {
			return nil, fmt.Errorf("enable layered mode: %w: %w", errs.ErrStoreWrite, err)
		}
		if len(volumes) > 0 && len(volumes[0].Arcs) > 0 {
			if err := t.store.Progress.UpdateVolumeArc(volumes[0].Index, volumes[0].Arcs[0].Index); err != nil {
				return nil, fmt.Errorf("set initial volume/arc: %w: %w", errs.ErrStoreWrite, err)
			}
		}
		result["volumes"] = len(volumes)
		result["dynamic_planning"] = true
		result["outlined_chapters"] = len(domain.FlattenOutline(volumes))

	case "characters":
		var chars []domain.Character
		if err := decode("characters", &chars); err != nil {
			return nil, err
		}
		// Gói S (06-data-contract › Bổ sung §S): mỗi nhân vật mang `kind` trong
		// lead/antagonist/ally/support — Kiến trúc sư điền; thiếu thì store suy từ `role`.
		for i, c := range chars {
			if c.Kind != "" && !domain.ValidCharacterKind(c.Kind) {
				return nil, fmt.Errorf("characters[%d] %q: kind %q không hợp lệ, phải là lead|antagonist|ally|support: %w",
					i, c.Name, c.Kind, errs.ErrToolArgs)
			}
		}
		if err := t.store.Characters.Save(chars); err != nil {
			return nil, fmt.Errorf("save characters: %w: %w", errs.ErrStoreWrite, err)
		}
		if len(a.AddressTerms) > 0 {
			if err := resolveAddressInputs(t.store, a.AddressTerms); err != nil {
				return nil, err
			}
			if err := t.store.World.UpdateAddressTerms(1, a.AddressTerms); err != nil {
				return nil, fmt.Errorf("lưu xưng hô nền: %v: %w", err, errs.ErrToolArgs)
			}
		}
		result["count"] = len(chars)

	case "world_rules":
		var rules []domain.WorldRule
		if err := decode("world_rules", &rules); err != nil {
			return nil, err
		}
		if err := t.store.World.SaveWorldRules(rules); err != nil {
			return nil, fmt.Errorf("save world_rules: %w: %w", errs.ErrStoreWrite, err)
		}
		result["count"] = len(rules)

	case "expand_arc":
		if a.Volume <= 0 || a.Arc <= 0 {
			return nil, fmt.Errorf(
				"expand_arc cần volume và arc (số tập/số cung đều tính từ 1, nhận được volume=%d arc=%d); "+
					"hãy gọi novel_context trước, dùng giá trị index của cung mục tiêu trong layered_outline để điền: %w",
				a.Volume, a.Arc, errs.ErrToolArgs)
		}
		var expansion domain.ArcExpansion
		if err := decode("expand_arc", &expansion); err != nil {
			return nil, err
		}
		if defect := domain.OversizedArc(
			fmt.Sprintf("tập %d cung %d", a.Volume, a.Arc), len(expansion.Chapters)); defect != "" {
			return nil, fmt.Errorf("%s: %w", defect, errs.ErrToolArgs)
		}
		if err := validateRetention(t.store, t.pack, json.RawMessage(content), false); err != nil {
			return nil, err
		}
		if err := t.store.ExpandArc(a.Volume, a.Arc, expansion); err != nil {
			return nil, fmt.Errorf("expand arc: %w: %w", errs.ErrStoreWrite, err)
		}
		result["volume"] = a.Volume
		result["arc"] = a.Arc
		result["title"] = expansion.Title
		result["goal"] = expansion.Goal
		result["chapters"] = len(expansion.Chapters)
		if err := t.consumeWriterFeedback(); err != nil {
			return nil, err
		}

	case "append_volume":
		p, err := t.store.Progress.Load()
		if err != nil {
			return nil, fmt.Errorf("load progress: %w: %w", errs.ErrStoreRead, err)
		}
		if p != nil && p.Phase == domain.PhaseComplete {
			return nil, fmt.Errorf("sách đã hoàn tất (phase=complete), không được thêm tập mới: %w", errs.ErrToolPrecondition)
		}
		var vol domain.VolumeOutline
		if err := decode("append_volume", &vol); err != nil {
			return nil, err
		}
		for i := range vol.Arcs {
			arc := &vol.Arcs[i]
			if defect := domain.OversizedArc(
				fmt.Sprintf("tập mới cung %d", arc.Index), arcPlannedSize(arc)); defect != "" {
				return nil, fmt.Errorf("%s: %w", defect, errs.ErrToolArgs)
			}
		}
		if err := validateRetention(t.store, t.pack, json.RawMessage(content), false); err != nil {
			return nil, err
		}
		prior, err := t.store.Outline.LoadLayeredOutline()
		if err != nil {
			return nil, fmt.Errorf("load layered outline: %w: %w", errs.ErrStoreRead, err)
		}
		if err := t.store.AppendVolume(vol); err != nil {
			return nil, fmt.Errorf("append volume: %w: %w", errs.ErrStoreWrite, err)
		}
		result["volume"] = vol.Index
		if vol.Final {
			result["final_volume"] = true
		} else if domain.FinaleVolume(prior) > 0 {
			// 事实回显：此前宣告的收官态因追加普通新卷而解除（新卷成为末卷）
			result["finale_released"] = true
		}
		result["arcs"] = len(vol.Arcs)
		chCount := 0
		for _, arc := range vol.Arcs {
			chCount += len(arc.Chapters)
		}
		if chCount > 0 {
			result["chapters"] = chCount
		}
		if err := t.consumeWriterFeedback(); err != nil {
			return nil, err
		}

	case "complete_book":
		// 全书完结的唯一入口：直接推 Phase=Complete。
		// 仅 Writing 阶段允许，防止规划阶段误调跳过整本写作。
		// 拒绝有返工队列时调用——保证 PendingRewrites 跑完才能结束。
		progress, perr := t.store.Progress.Load()
		if perr != nil {
			return nil, fmt.Errorf("load progress: %w: %w", errs.ErrStoreRead, perr)
		}
		if progress == nil {
			return nil, fmt.Errorf("progress chưa được khởi tạo: %w", errs.ErrToolPrecondition)
		}
		if progress.Phase != domain.PhaseWriting {
			return nil, fmt.Errorf("complete_book chỉ gọi được ở giai đoạn writing (phase hiện tại=%s): %w", progress.Phase, errs.ErrToolPrecondition)
		}
		if len(progress.PendingRewrites) > 0 {
			return nil, fmt.Errorf("còn %d chương trong hàng đợi viết lại, xử lý xong rồi mới gọi complete_book: %w", len(progress.PendingRewrites), errs.ErrToolPrecondition)
		}
		// 可枚举的完本前置校验必须在代码层(三分法),不能只依赖提示词里的
		// "完结判定清单"——真实事故:规划刚落盘 phase 翻到 writing,弱模型顺手
		// 误调 complete_book,0/68 章被直接标记完本。
		if len(progress.CompletedChapters) == 0 {
			return nil, fmt.Errorf("chưa viết chương nào thì không thể hoàn tất; sau khi quy hoạch xong việc viết do hệ thống tự đẩy, không cần gọi complete_book: %w", errs.ErrToolPrecondition)
		}
		next := progress.NextChapter()
		if progress.Layered {
			outline, outlineErr := t.store.Outline.LoadOutline()
			if outlineErr != nil {
				return nil, fmt.Errorf("load outlined chapters: %w: %w", errs.ErrStoreRead, outlineErr)
			}
			if next <= len(outline) {
				return nil, fmt.Errorf("đại cương chi tiết hiện tại còn chương chưa viết (chương tiếp theo %d/đã chi tiết %d), không thể hoàn tất; muốn kết thúc sớm hãy dùng append_volume với JSON tập có \"final\": true ở tầng trên cùng để tuyên bố tập kết thúc: %w", next, len(outline), errs.ErrToolPrecondition)
			}
			// 扁平大纲只含已展开的弧，骨架弧对上面的比对完全隐形；不单独拦一次，
			// 整卷未展开也能宣告完本。
			volumes, volErr := t.store.Outline.LoadLayeredOutline()
			if volErr != nil {
				return nil, fmt.Errorf("load layered outline: %w: %w", errs.ErrStoreRead, volErr)
			}
			if skeletons := domain.SkeletonArcs(volumes); len(skeletons) > 0 {
				return nil, fmt.Errorf("còn %d cung khung chưa mở rộng (ví dụ: %s), không thể hoàn tất; "+
					"hãy expand_arc để mở rộng và viết xong, hoặc dùng append_volume kèm \"final\": true để tuyên bố tập kết thúc: %w",
					len(skeletons), skeletons[0], errs.ErrToolPrecondition)
			}
		} else if progress.TotalChapters > 0 && next <= progress.TotalChapters {
			return nil, fmt.Errorf("đại cương còn chương chưa viết (chương tiếp theo %d/tổng %d), không thể hoàn tất; muốn kết thúc sớm hãy dùng append_volume với JSON tập có \"final\": true ở tầng trên cùng để tuyên bố tập kết thúc: %w", next, progress.TotalChapters, errs.ErrToolPrecondition)
		}
		// 活跃长线未收束不可完本——OpenThreads 的字段契约即"需收束才能结局"。这不是
		// 语义复判：真认为已全部收束，先 update_compass 清空 open_threads 再完本，把
		// "论述里豁免"变成可审计的落盘动作（实测导入完本书续写时，架构师引经据典绕过
		// 完结清单第 3 条直接完本，用户的续写诉求被完本规则锁死）。
		compass, err := t.store.Outline.LoadCompass()
		if err != nil {
			return nil, fmt.Errorf("load compass: %w: %w", errs.ErrStoreRead, err)
		}
		if compass != nil && len(compass.OpenThreads) > 0 {
			return nil, fmt.Errorf("compass còn %d tuyến dài đang hoạt động chưa khép (ví dụ: %s), không thể hoàn tất. Nếu xác nhận đã khép hết, hãy update_compass làm rỗng open_threads rồi mới gọi complete_book; nếu vẫn cần mở rộng hãy append_volume (có thể kèm \"final\": true để tuyên bố tập kết thúc): %w",
				len(compass.OpenThreads), compass.OpenThreads[0], errs.ErrToolPrecondition)
		}
		if err := t.store.Progress.MarkComplete(); err != nil {
			return nil, fmt.Errorf("mark complete: %w: %w", errs.ErrStoreWrite, err)
		}
		result["book_complete"] = true
		result["phase"] = string(domain.PhaseComplete)

	case "update_compass":
		var compass domain.StoryCompass
		if err := decode("compass", &compass); err != nil {
			return nil, err
		}
		// 工具层强制覆盖 LastUpdated 为当前已完成章节数，不信任 LLM 自填。
		// LLM 通常忘填或留 0，会让 diag.CompassDrift 误报、Router 路由失真。
		p, err := t.store.Progress.Load()
		if err != nil {
			return nil, fmt.Errorf("load progress: %w: %w", errs.ErrStoreRead, err)
		}
		if p != nil {
			compass.LastUpdated = p.LatestCompleted()
		}
		if err := t.store.Outline.SaveCompass(compass); err != nil {
			return nil, fmt.Errorf("save compass: %w: %w", errs.ErrStoreWrite, err)
		}
		result["ending_direction"] = compass.EndingDirection
		result["last_updated"] = compass.LastUpdated
		if err := t.consumeWriterFeedback(); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unknown type %q, expected premise/outline/layered_outline/characters/world_rules/expand_arc/append_volume/update_compass/complete_book: %w", a.Type, errs.ErrToolArgs)
	}
	if t.pack != nil && (a.Type == "premise" || a.Type == "outline" || a.Type == "layered_outline" || a.Type == "expand_arc" || a.Type == "append_volume") {
		var requirements retentionRequirements
		if _, err := t.pack.Section("giu_chan", &requirements); err != nil {
			return nil, err
		}
		fields, err := t.store.Outline.LoadStoryRetention()
		if err != nil {
			return nil, fmt.Errorf("load story retention warnings: %w", err)
		}
		for _, key := range requirements.Warnings {
			if key == "hat_giong_phan_hai" && strings.TrimSpace(fields.HatGiongPhanHai) == "" {
				result["warnings"] = []string{"[CẢNH BÁO] Thiếu hat_giong_phan_hai (Master §19): ghi câu hỏi mở phần hai mà không phá kết truyện này."}
			}
		}
	}

	// checkpoint
	scope := domain.GlobalScope()
	if a.Type == "expand_arc" {
		scope = domain.ArcScope(a.Volume, a.Arc)
	} else if a.Type == "append_volume" {
		scope = domain.GlobalScope()
	}
	if _, err := t.store.Checkpoints.AppendArtifact(scope, a.Type, foundationArtifact(a.Type)); err != nil {
		return nil, fmt.Errorf("checkpoint foundation %s: %w: %w", a.Type, errs.ErrStoreWrite, err)
	}

	if volumeEnd {
		t.recordVolumeEndDecision(a.Type, a.Reason, volumeEndFacts, result)
	}

	// 返回剩余未完成项。初始工件齐全后仍会剩 foundation_audit；只有
	// audit_foundation 对实际落盘版本给出 ready=true，才允许进入 writing。
	remaining, err := t.store.FoundationMissing()
	if err != nil {
		return nil, fmt.Errorf("load foundation state: %w: %w", errs.ErrStoreRead, err)
	}
	ready := len(remaining) == 0
	result["remaining"] = remaining
	result["foundation_ready"] = ready
	return json.Marshal(result)
}

func foundationArtifact(t string) string {
	switch t {
	case "premise":
		return "premise.md"
	case "outline":
		return "outline.json"
	case "layered_outline", "expand_arc", "append_volume":
		return "layered_outline.json"
	case "complete_book":
		return "meta/progress.json"
	case "characters":
		return "characters.json"
	case "world_rules":
		return "world_rules.json"
	case "update_compass":
		return "meta/compass.json"
	default:
		return ""
	}
}

// decodeFoundationJSON 解析 save_foundation 的 content 字段，失败时附上行列位置
// 和最常见的修复提示，让 LLM 下一次重试能直接定位而不是盲猜。
func decodeFoundationJSON(typeName, content string, out any) error {
	err := json.Unmarshal([]byte(content), out)
	if err == nil {
		return nil
	}
	hint := `Nguyên nhân thường gặp: dấu ngoặc kép trong giá trị chuỗi chưa escape thành \", xuống dòng chưa escape thành \n, hoặc thiếu dấu phẩy giữa các trường của object. Hãy tạo lại toàn bộ một lần.`
	if se, ok := err.(*json.SyntaxError); ok {
		line, col := offsetToLineCol(content, int(se.Offset))
		return fmt.Errorf("parse %s JSON (line %d col %d): %w — %s", typeName, line, col, err, hint)
	}
	return fmt.Errorf("parse %s JSON: %w — %s", typeName, err, hint)
}

func offsetToLineCol(s string, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(s) {
		offset = len(s)
	}
	line, col := 1, 1
	for i := 0; i < offset; i++ {
		if s[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

func normalizeFoundationContent(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("content is required: %w", errs.ErrToolArgs)
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}

	if !json.Valid(raw) {
		return "", fmt.Errorf("invalid content: expected Markdown string or valid JSON value: %w", errs.ErrToolArgs)
	}
	return string(raw), nil
}

// recordVolumeEndDecision 把卷末三选一（续卷/收官/完结）的判定理由落进裁定审计。
// best-effort：结构变更已落盘，审计失败只告警不回滚——报错会让模型重试已完成
// 的操作（重复追加卷）。
func (t *SaveFoundationTool) recordVolumeEndDecision(action, reason string, facts json.RawMessage, result map[string]any) {
	decision := map[string]any{"action": action}
	if v, ok := result["volume"]; ok {
		decision["volume"] = v
	}
	if _, ok := result["final_volume"]; ok {
		decision["final"] = true
	}
	raw, err := json.Marshal(decision)
	if err != nil {
		slog.Error("tuần tự hoá phán định cuối tập thất bại", "module", "tools", "action", action, "err", err)
		return
	}
	if _, err := t.store.Decisions.Append(store.DecisionRecord{
		Kind:     "volume_end",
		Decider:  "architect",
		Facts:    facts,
		Decision: raw,
		Reason:   reason,
	}); err != nil {
		slog.Error("ghi nhật ký phán định cuối tập thất bại", "module", "tools", "action", action, "err", err)
	}
}

// consumeWriterFeedback 在结构操作成功后清除已处理的规划反馈。
func (t *SaveFoundationTool) consumeWriterFeedback() error {
	if err := t.store.Outline.ClearOutlineFeedback(); err != nil {
		return fmt.Errorf("clear outline feedback: %w: %w", errs.ErrStoreWrite, err)
	}
	return nil
}

// arcPlannedSize 取弧的实际规模：已展开用详细章节数，仍是骨架则用预估章数。
func arcPlannedSize(arc *domain.ArcOutline) int {
	if n := len(arc.Chapters); n > 0 {
		return n
	}
	return arc.EstimatedChapters
}
