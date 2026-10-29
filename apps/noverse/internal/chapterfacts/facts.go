package chapterfacts

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/datnt1998/noverse/internal/agent/schema"
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/llmcontract"
)

// Properties 返回完整章节事实共用的 JSON Schema 字段。
func Properties(includeFeedback bool) []schema.Prop {
	textList := func(description string) map[string]any {
		return schema.Array(description, schema.String(description))
	}
	quote := llmcontract.Nullable(schema.String(
		"Trích nguyên văn câu/đoạn trong chương chứng minh mục này (ít nhất 6 ký tự); null khi không cung cấp"))
	timeline := schema.Object(
		schema.Property("time", schema.String("Thời gian trong truyện")).Required(),
		schema.Property("event", schema.String("Sự kiện")).Required(),
		schema.Property("characters", textList("Nhân vật liên quan")).Required(),
		schema.Property("quote", quote).Required(),
	)
	foreshadow := schema.Object(
		schema.Property("id", schema.String("ID phục bút")).Required(),
		schema.Property("action", schema.Enum("Hành động", "plant", "advance", "resolve", "abandon")).Required(),
		schema.Property("description", llmcontract.Nullable(schema.String("Mô tả khi plant, các thao tác khác là null"))).Required(),
		schema.Property("due_by", llmcontract.Nullable(schema.Int(
			"Chương dự kiến thu phục bút (chỉ dùng khi plant, phải sau chương này); null khi không đặt mốc"))).Required(),
		schema.Property("reason", llmcontract.Nullable(schema.String(
			"Lý do bỏ tuyến (chỉ dùng khi abandon, bắt buộc); các thao tác khác là null"))).Required(),
		schema.Property("quote", quote).Required(),
	)
	relationship := schema.Object(
		schema.Property("character_a", schema.String("Nhân vật A")).Required(),
		schema.Property("character_b", schema.String("Nhân vật B")).Required(),
		schema.Property("relation", schema.String("Quan hệ ở cuối chương này, mô tả ngắn bằng tiếng Việt")).Required(),
		schema.Property("type", llmcontract.Nullable(schema.Enum(
			"Kiểu quan hệ để vẽ Não truyện: enemy = đối đầu, ally = đồng minh, work = công việc, "+
				"own = sở hữu/tác giả, secret = ẩn chưa lộ, rel = liên quan khác; null khi không phân loại được",
			domain.RelationshipTypes()...))).Required(),
	)
	stateChange := schema.Object(
		schema.Property("entity", schema.String("Thực thể")).Required(),
		schema.Property("field", schema.String("Thuộc tính")).Required(),
		schema.Property("old_value", llmcontract.Nullable(schema.String("Giá trị trước khi đổi"))).Required(),
		schema.Property("new_value", schema.String("Giá trị sau khi đổi")).Required(),
		schema.Property("reason", llmcontract.Nullable(schema.String("Nguyên nhân"))).Required(),
		schema.Property("quote", quote).Required(),
	)
	// places/objects (GAP-49): Não truyện vẽ thành nút Địa điểm / Vật. Cùng hình dạng,
	// khác nghĩa, nên khai hai schema riêng để mô tả tiếng Việt nói đúng thứ model phải điền.
	placeLike := func(kind, what string) map[string]any {
		return schema.Object(
			schema.Property("name", schema.String("Tên "+kind+", dùng lại đúng tên đã ghi ở chương trước nếu đã có")).Required(),
			schema.Property("desc", schema.String("Một câu mô tả "+kind+" này trong truyện")).Required(),
			schema.Property("firstChapter", llmcontract.Nullable(schema.Int("Chương "+kind+" xuất hiện lần đầu; null nếu chính chương này là lần đầu"))).Required(),
			schema.Property("with", schema.Array("Tên những gì đi kèm "+kind+" trong chương này (Não truyện nối thành cạnh)",
				schema.String("Tên nhân vật đúng characters.json, hoặc tên một "+what+" khác đã ghi"))).Required(),
		)
	}
	props := []schema.Prop{
		schema.Property("title", schema.String("Tiêu đề cuối cùng")).Required(),
		schema.Property("summary", schema.String("Tóm tắt chương: chỉ kể việc đã xảy ra trong chương, không ghi dự định hay suy đoán")).Required(),
		schema.Property("characters", textList("Nhân vật xuất hiện")).Required(),
		schema.Property("key_events", textList("Sự kiện then chốt đã xảy ra trong chương; dự định, nghi ngờ, phủ nhận hay sự kiện bị hủy không ghi vào đây")).Required(),
		schema.Property("timeline_events", schema.Array("Sự kiện dòng thời gian đã thực sự xảy ra trong chương; `time` quy đổi từ chỉ thời gian tương đối theo mốc đã rõ trong chính văn; không ghi dự định, nghi ngờ, phủ nhận hay sự kiện bị hủy", timeline)).Required(),
		schema.Property("foreshadow_updates", schema.Array("Thao tác phục bút", foreshadow)).Required(),
		schema.Property("relationship_changes", schema.Array("Thay đổi quan hệ", relationship)).Required(),
		schema.Property("state_changes", schema.Array("Thay đổi trạng thái đã thực sự xảy ra trong chương; có đính chính trong cùng chương thì ghi bản sau cùng; ghi rõ ai biết thông tin, ai sở hữu vật", stateChange)).Required(),
		schema.Property("cast_intros", schema.Array("Nhân vật phụ mới", schema.Object(
			schema.Property("name", schema.String("Tên")).Required(),
			schema.Property("brief_role", schema.String("Vai trò")).Required(),
		))).Required(),
		schema.Property("places", llmcontract.Nullable(schema.Array(
			"Địa điểm chương này dùng tới: nơi cảnh xảy ra, kể cả nơi đã xuất hiện ở chương trước."+
				" Mỗi địa điểm một mục, ghi đúng tên đã dùng ở chương trước; không có thì null hoặc [].", placeLike("địa điểm", "địa điểm")))).Required(),
		schema.Property("objects", llmcontract.Nullable(schema.Array(
			"Vật chương này dùng tới: đồ vật có vai trò trong cảnh, kể cả vật đã xuất hiện ở chương trước."+
				" Mỗi vật một mục, ghi đúng tên đã dùng ở chương trước; không có thì null hoặc [].", placeLike("vật", "vật")))).Required(),
		schema.Property("hook_type", llmcontract.Nullable(schema.Enum("Điểm móc cuối chương", domain.HookTypes()...))).Required(),
		schema.Property("dominant_strand", llmcontract.Nullable(schema.Enum("Tuyến trần thuật chủ đạo", domain.DominantStrands()...))).Required(),
	}
	if includeFeedback {
		feedback := schema.Object(
			schema.Property("deviation", schema.String("Mô tả chỗ lệch khỏi đại cương")).Required(),
			schema.Property("suggestion", schema.String("Đề xuất điều chỉnh đại cương phía sau")).Required(),
		)
		feedback["description"] = "Đối tượng đề xuất cho đại cương phía sau; phải truyền thẳng JSON object, không truyền JSON dạng chuỗi"
		props = append(props, schema.Property("feedback", llmcontract.Nullable(feedback)).Required())
	}
	return props
}

// Validate 校验普通提交与人工修订共用的确定性约束。
func Validate(facts domain.ChapterFacts) error {
	if strings.TrimSpace(facts.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(facts.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	if len(facts.KeyEvents) == 0 {
		return fmt.Errorf("key_events must contain at least one event")
	}
	if err := validateTextItems("characters", facts.Characters); err != nil {
		return err
	}
	if err := validateTextItems("key_events", facts.KeyEvents); err != nil {
		return err
	}
	for i, event := range facts.TimelineEvents {
		if strings.TrimSpace(event.Time) == "" || strings.TrimSpace(event.Event) == "" {
			return fmt.Errorf("timeline_events[%d] requires time and event", i)
		}
		if err := validateTextItems(fmt.Sprintf("timeline_events[%d].characters", i), event.Characters); err != nil {
			return err
		}
	}
	for i, update := range facts.ForeshadowUpdates {
		if strings.TrimSpace(update.ID) == "" {
			return fmt.Errorf("foreshadow_updates[%d].id is required", i)
		}
		if placeholderForeshadowID(update.ID) {
			return fmt.Errorf(
				"foreshadow_updates[%d].id = %q là giá trị giữ chỗ, không phải định danh. Mỗi phục bút cần một id duy nhất và ổn định"+
					" (ví dụ FORESHADOW_OBSERVER, jade-pendant), các chương sau dựa vào nó để advance/resolve; "+
					"dùng lại giá trị giữ chỗ sẽ khiến phục bút mới đâm vào bản ghi cũ",
				i, update.ID)
		}
		switch update.Action {
		case "plant":
			if strings.TrimSpace(update.Description) == "" {
				return fmt.Errorf("foreshadow_updates[%d] plant requires description", i)
			}
		case "abandon":
			if strings.TrimSpace(update.Reason) == "" {
				return fmt.Errorf("foreshadow_updates[%d] abandon requires reason", i)
			}
		case "advance", "resolve":
		default:
			return fmt.Errorf("foreshadow_updates[%d].action invalid: %q", i, update.Action)
		}
	}
	for i, change := range facts.RelationshipChanges {
		if strings.TrimSpace(change.CharacterA) == "" || strings.TrimSpace(change.CharacterB) == "" || strings.TrimSpace(change.Relation) == "" {
			return fmt.Errorf("relationship_changes[%d] requires character_a, character_b and relation", i)
		}
		if change.CharacterA == change.CharacterB {
			return fmt.Errorf("relationship_changes[%d] cannot relate a character to itself", i)
		}
		if change.Type != "" && !domain.ValidRelationshipType(change.Type) {
			return fmt.Errorf("relationship_changes[%d].type invalid: %q", i, change.Type)
		}
	}
	for i, change := range facts.StateChanges {
		if strings.TrimSpace(change.Entity) == "" || strings.TrimSpace(change.Field) == "" || strings.TrimSpace(change.NewValue) == "" {
			return fmt.Errorf("state_changes[%d] requires entity, field and new_value", i)
		}
	}
	for i, intro := range facts.CastIntros {
		if strings.TrimSpace(intro.Name) == "" || strings.TrimSpace(intro.BriefRole) == "" {
			return fmt.Errorf("cast_intros[%d] requires name and brief_role", i)
		}
	}
	if err := validateFactEntities("places", facts.Places); err != nil {
		return err
	}
	if err := validateFactEntities("objects", facts.Objects); err != nil {
		return err
	}
	if facts.HookType != "" && !domain.ValidHookType(facts.HookType) {
		return fmt.Errorf("invalid hook_type %q", facts.HookType)
	}
	if facts.DominantStrand != "" && !domain.ValidDominantStrand(facts.DominantStrand) {
		return fmt.Errorf("invalid dominant_strand %q", facts.DominantStrand)
	}
	if facts.Feedback != nil && (strings.TrimSpace(facts.Feedback.Deviation) == "" || strings.TrimSpace(facts.Feedback.Suggestion) == "") {
		return fmt.Errorf("feedback requires deviation and suggestion")
	}
	return nil
}

// validateFactEntities kiểm địa điểm/vật của chương (GAP-49). Tên là thứ duy nhất Não
// truyện dùng để cấp mã ổn định qua các chương, nên không được rỗng; FirstChapter = 0
// nghĩa là chính chương đang nộp, số âm là dữ liệu sai.
func validateFactEntities(name string, items []domain.FactEntity) error {
	for i, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("%s[%d] requires name", name, i)
		}
		if item.FirstChapter < 0 {
			return fmt.Errorf("%s[%d].firstChapter cannot be negative", name, i)
		}
		if err := validateTextItems(fmt.Sprintf("%s[%d].with", name, i), item.With); err != nil {
			return err
		}
	}
	return nil
}

func validateTextItems(name string, items []string) error {
	for i, item := range items {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("%s[%d] cannot be empty", name, i)
		}
	}
	return nil
}

// placeholderForeshadowIDs 是模型常写进 id 字段的占位词。它们本身没有标识作用，
// 而伏笔 id 必须跨章稳定——实测模型把每条新伏笔都命名为 "new"，第二条起就撞上
// 第一条，被静默丢弃：22 章里 plant 了 6 次，最终只入账 2 条。
var placeholderForeshadowIDs = map[string]struct{}{
	"new": {}, "none": {}, "null": {}, "nil": {}, "tbd": {}, "todo": {},
	"auto": {}, "id": {}, "-": {}, "n/a": {}, "na": {}, "unknown": {}, "?": {},
}

func placeholderForeshadowID(id string) bool {
	_, ok := placeholderForeshadowIDs[strings.ToLower(strings.TrimSpace(id))]
	return ok
}

// ValidateQuotes 校验所有非空引文能在给定正文中定位：引文是可选证据，但
// 一旦提供就必须对应本章正文的原句，否则下游无法据此回取原文段落。
// 只在校验方持有正文时调用——Validate 本身只做无文本的确定性检查，
// 载荷恢复与导入流程不经过这里。
func ValidateQuotes(text string, facts domain.ChapterFacts) error {
	check := func(label, quote string) error {
		quote = strings.TrimSpace(quote)
		if quote == "" {
			return nil
		}
		if utf8.RuneCountInString(quote) < domain.MinQuoteRunes {
			return fmt.Errorf("%s: quote phải có ít nhất %d ký tự để định vị được", label, domain.MinQuoteRunes)
		}
		if _, _, ok := domain.LocateQuote(text, quote); ok {
			return nil
		}
		if snippet, found := domain.NearestQuotePrefix(text, quote); found {
			return fmt.Errorf("%s: quote không tìm thấy trong chính văn; đoạn gần nhất trong chính văn: %q — hãy chép nguyên văn", label, snippet)
		}
		return fmt.Errorf("%s: quote không tìm thấy trong chính văn", label)
	}
	for i, event := range facts.TimelineEvents {
		if err := check(fmt.Sprintf("timeline_events[%d]", i), event.Quote); err != nil {
			return err
		}
	}
	for i, update := range facts.ForeshadowUpdates {
		if err := check(fmt.Sprintf("foreshadow_updates[%d] (%s)", i, update.ID), update.Quote); err != nil {
			return err
		}
	}
	for i, change := range facts.StateChanges {
		if err := check(fmt.Sprintf("state_changes[%d] (%s.%s)", i, change.Entity, change.Field), change.Quote); err != nil {
			return err
		}
	}
	return nil
}
