package domain

// PackRuleItem giữ nguyên nội dung từng gạch đầu dòng và mục gần nhất chứa nó.
type PackRuleItem struct {
	Ref   string
	Label string // LỖI / CẢNH BÁO / rỗng
	Text  string
}
