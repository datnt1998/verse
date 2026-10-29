package sim

import (
	"github.com/datnt1998/verse/apps/noverse/internal/agent/schema"
	"github.com/datnt1998/verse/apps/noverse/internal/llmcontract"
)

func textList(description string) map[string]any {
	return schema.Array(description, schema.String(description))
}

var sourceReportContract = llmcontract.Contract{
	Name:        "simulation_source_report",
	Description: "Từ ngữ liệu một tác phẩm đơn lẻ, chắt lọc phương pháp viết có thể tái sử dụng mà không sao chép nguyên văn",
	Schema: schema.Object(
		schema.Property("title", llmcontract.Nullable(schema.String("Tiêu đề tuỳ chọn; null nếu không thể xác nhận"))).Required(),
		schema.Property("summary", schema.String("Khái quát giá trị cách viết của văn bản mẫu")).Required(),
		schema.Property("style_observations", textList("Quan sát góc nhìn tự sự, cấu trúc câu và kết cấu miêu tả")).Required(),
		schema.Property("common_words", textList("Nhóm từ tần suất cao, hình tượng và từ chuyển tiếp")).Required(),
		schema.Property("plot_patterns", textList("Mô thức đẩy tiến cốt truyện, bước ngoặt và leo thang xung đột")).Required(),
		schema.Property("hook_patterns", textList("Mô thức điểm móc mở đầu, kết chương và chênh lệch thông tin")).Required(),
		schema.Property("pacing_notes", textList("Mật độ cảnh và nhịp tiết lộ thông tin")).Required(),
		schema.Property("reader_appeal", textList("Phương pháp thu hút độc giả đọc tiếp")).Required(),
		schema.Property("reusable_techniques", textList("Kỹ thuật cấu trúc có thể học hỏi")).Required(),
		schema.Property("warnings", textList("Rủi ro sao chép và áp dụng máy móc bắt buộc phải tránh")).Required(),
	),
}

var synthesisContract = llmcontract.Contract{
	Name:        "simulation_synthesis",
	Description: "Tổng hợp hồ sơ hiện có và báo cáo ngữ liệu thành hồ sơ phương pháp mô phỏng văn phong có thể thực thi",
	Schema: schema.Object(
		schema.Property("style", schema.Object(
			schema.Property("narrative_voice", textList("Ngôi kể, khoảng cách và kiểm soát thông tin")).Required(),
			schema.Property("sentence_rhythm", textList("Nhịp câu")).Required(),
			schema.Property("prose_texture", textList("Chất lượng miêu tả")).Required(),
			schema.Property("perspective", textList("Quy tắc góc nhìn")).Required(),
			schema.Property("mood", textList("Sắc thái cảm xúc")).Required(),
			schema.Property("do_not_copy", textList("Nội dung cấm sao chép")).Required(),
		)).Required(),
		schema.Property("lexicon", schema.Object(
			schema.Property("common_words", textList("Nhóm từ thường dùng")).Required(),
			schema.Property("emotion_words", textList("Nhóm từ cảm xúc")).Required(),
			schema.Property("scene_words", textList("Nhóm từ cảnh")).Required(),
			schema.Property("transition_words", textList("Nhóm từ chuyển tiếp")).Required(),
			schema.Property("signature_phrases", textList("Đặc trưng giọng điệu đã trừu tượng hoá, không chứa câu gốc")).Required(),
		)).Required(),
		schema.Property("plot_design", schema.Object(
			schema.Property("opening_patterns", textList("Cách mở đầu")).Required(),
			schema.Property("escalation_patterns", textList("Cách leo thang xung đột")).Required(),
			schema.Property("turning_point_patterns", textList("Thiết kế bước ngoặt")).Required(),
			schema.Property("payoff_patterns", textList("Cách thu hồi và trả nợ")).Required(),
		)).Required(),
		schema.Property("hook_design", schema.Object(
			schema.Property("hook_types", textList("Loại điểm móc")).Required(),
			schema.Property("placement", textList("vị trí điểm móc")).Required(),
			schema.Property("cliffhanger_patterns", textList("cách tạo điểm dừng hồi hộp")).Required(),
			schema.Property("payoff_rules", textList("quy tắc trả điểm móc")).Required(),
		)).Required(),
		schema.Property("pacing_density", schema.Object(
			schema.Property("scene_density", textList("mật độ thông tin trong một cảnh")).Required(),
			schema.Property("information_release", textList("nhịp tiết lộ thông tin")).Required(),
			schema.Property("dialogue_action_ratio", textList("tỷ lệ đối thoại, hành động và tâm lý")).Required(),
			schema.Property("compression_rules", textList("quy tắc mở rộng và nén nội dung")).Required(),
		)).Required(),
		schema.Property("reader_engagement", schema.Object(
			schema.Property("methods", textList("phương pháp thu hút độc giả")).Required(),
			schema.Property("emotional_drivers", textList("động lực cảm xúc")).Required(),
			schema.Property("progression_rewards", textList("phần thưởng tiến triển theo giai đoạn")).Required(),
			schema.Property("anti_patterns", textList("phản mẫu làm giảm sức hút")).Required(),
		)).Required(),
		schema.Property("role_guidance", schema.Object(
			schema.Property("architect", textList("quy tắc Architect sử dụng hồ sơ mô phỏng")).Required(),
			schema.Property("writer", textList("quy tắc Writer học hỏi nhưng không sao chép")).Required(),
			schema.Property("editor", textList("quy tắc Editor kiểm tra phương hướng và rủi ro vi phạm")).Required(),
		)).Required(),
	),
}
