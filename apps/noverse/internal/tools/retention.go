package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/datnt1998/noverse/internal/agent/schema"
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/errs"
	"github.com/datnt1998/noverse/internal/llmcontract"
	"github.com/datnt1998/noverse/internal/store"
)

const hookTypeDescription = "Kiểu móc theo Master §7: lat_thong_tin (lật thông tin), de_doa_moi (đe dọa mới), quyet_dinh_treo (quyết định treo), cau_thoai_no (câu thoại nổ); chỉ chương cuối mới dùng seed (hạt giống phần hai). Không ghi các kiểu móc cũ."
const promiseDescription = "Lời hứa/đếm ngược theo Master §3. Chỉ khi cảnh hứa ở tiêu đề hoặc mở đầu được diễn trọn, dùng tiền tố da_tra: rồi mô tả cảnh; không dùng cho trả thưởng phụ. Chương da_tra: đầu tiên là cảnh lời hứa."

// RetentionPack 提供 gói.json 分节读取，避免 tools 依赖 assets。
type RetentionPack interface {
	Section(string, any) (bool, error)
}

func retentionProperties(strict bool) []schema.Prop {
	property := func(name string, shape map[string]any) schema.Prop {
		if strict {
			return schema.Property(name, llmcontract.Nullable(shape)).Required()
		}
		return schema.Property(name, shape)
	}
	return []schema.Prop{
		property("open_questions", schema.Array("Các câu hỏi còn treo; chương kết có thể là mảng rỗng; null khi không dùng bản đồ giữ chân", schema.String(""))),
		property("promise", schema.String(promiseDescription)),
		property("hook_type", schema.Enum(hookTypeDescription, "lat_thong_tin", "de_doa_moi", "quyet_dinh_treo", "cau_thoai_no", "seed")),
		property("midpoint_twist", schema.Bool("Có cú lật giữa truyện ở chương này; các chương khác ghi false; null khi không dùng bản đồ giữ chân")),
	}
}

type retentionRequirements struct {
	Required []string `json:"bat_buoc"`
	Warnings []string `json:"canh_bao"`
}

func validateRetention(st *store.Store, pack RetentionPack, raw json.RawMessage, chapterList bool) error {
	var content any
	if err := json.Unmarshal(raw, &content); err != nil {
		return fmt.Errorf("đọc bản đồ giữ chân: %w", err)
	}
	if err := validateNewHookTypes(content); err != nil {
		return err
	}
	if pack == nil {
		return nil
	}
	var requirements retentionRequirements
	if _, err := pack.Section("giu_chan", &requirements); err != nil {
		return err
	}
	if len(requirements.Required) == 0 {
		return nil
	}
	fields, err := st.Outline.LoadStoryRetention()
	if err != nil {
		return fmt.Errorf("đọc trường cấp truyện: %w", err)
	}
	var missing []string
	for _, key := range requirements.Required {
		value := ""
		switch key {
		case "nguoi_nghe":
			if len(fields.NguoiNghe) > 0 {
				value = "selected"
			}
		case "diem_khac":
			value = domain.StoryDifference(fields.DiemKhac)
			if value == "" {
				others, err := siblingStories(st)
				if err != nil {
					return fmt.Errorf("đọc truyện để đối chiếu: %w", err)
				}
				if len(others) == 0 {
					continue
				}
			}
		case "hat_giong_phan_hai":
			value = fields.HatGiongPhanHai
		default:
			continue
		}
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key+" (save_foundation type=premise)")
		}
	}
	var visit func(any, string, bool)
	visit = func(node any, path string, chapters bool) {
		switch n := node.(type) {
		case []any:
			for i, item := range n {
				visit(item, fmt.Sprintf("%s[%d]", path, i+1), chapters)
			}
		case map[string]any:
			if chapters {
				for _, key := range requirements.Required {
					value, exists := n[key]
					valid := exists && value != nil
					switch key {
					case "open_questions":
						_, valid = value.([]any)
					case "promise", "hook_type":
						text, ok := value.(string)
						valid = ok && strings.TrimSpace(text) != ""
					case "midpoint_twist":
						_, valid = value.(bool)
					default:
						continue
					}
					if !valid {
						missing = append(missing, path+"."+key)
					}
				}
			}
			for _, key := range []string{"chapters", "arcs"} {
				if child, ok := n[key]; ok {
					visit(child, path+"."+key, key == "chapters")
				}
			}
		}
	}
	visit(content, "outline", chapterList)
	if len(missing) > 0 {
		return fmt.Errorf("Bản đồ giữ chân còn thiếu: %s. Architect cần bổ sung rồi lưu lại: %w", strings.Join(missing, "; "), errs.ErrToolArgs)
	}
	return nil
}

func normalizeStoryRetention(fields domain.StoryRetention, pack RetentionPack) (domain.StoryRetention, error) {
	groups := domain.AudienceLabels
	fields.DiemKhac = domain.StoryDifference(fields.DiemKhac)
	fields.HatGiongPhanHai = strings.TrimSpace(fields.HatGiongPhanHai)
	var audience string
	if pack != nil {
		if _, err := pack.Section("nguoi_nghe", &audience); err != nil {
			return fields, err
		}
	}
	exactDefault := false
	for _, group := range groups {
		if audience == group {
			exactDefault = true
			if len(fields.NguoiNghe) == 0 {
				for i, label := range groups {
					if label == group {
						fields.NguoiNghe = []int{i}
						break
					}
				}
			}
		}
	}
	if len(fields.NguoiNghe) == 0 {
		return fields, nil
	}
	for _, index := range fields.NguoiNghe {
		if index < 0 || index >= len(groups) {
			return fields, fmt.Errorf("nguoi_nghe phải là chỉ số nhóm từ 0 đến 4: %w", errs.ErrToolArgs)
		}
		// 单一默认可被初始用户要求覆盖；复合声明必须从该声明中选择。
		if !exactDefault && audience != "" && !strings.Contains(strings.ToLower(audience), strings.ToLower(groups[index])) {
			return fields, fmt.Errorf("nguoi_nghe phải chọn một nhóm được gói nêu: %s: %w", audience, errs.ErrToolArgs)
		}
	}
	return fields, nil
}

// validateNewHookTypes 限制新写入的 hook_type；旧大纲仍由读取端映射。
// seed 的位置由读取端处理：非末章忽略并给出 §7 解释。
func validateNewHookTypes(content any) error {
	var visit func(any) error
	visit = func(node any) error {
		switch value := node.(type) {
		case []any:
			for _, child := range value {
				if err := visit(child); err != nil {
					return err
				}
			}
		case map[string]any:
			if hook, ok := value["hook_type"].(string); ok && hook != "" {
				switch hook {
				case "lat_thong_tin", "de_doa_moi", "quyet_dinh_treo", "cau_thoai_no", "seed":
				default:
					return fmt.Errorf("hook_type %q không hợp lệ; chỉ nhận lat_thong_tin, de_doa_moi, quyet_dinh_treo, cau_thoai_no, riêng chương cuối có thể dùng seed — sửa đúng trường này rồi lưu lại: %w", hook, errs.ErrToolArgs)
				}
			}
			for _, key := range []string{"chapters", "arcs"} {
				if err := visit(value[key]); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(content)
}
