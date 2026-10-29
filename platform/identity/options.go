package identity

// Options là phần định danh project mà app phải cung cấp khi dựng Store: hình
// dạng project hợp lệ và ID của project "legacy" gom các ID không tên miền.
// Hai trị này từng gắn cứng vào package library của một app cụ thể; đưa vào
// đây để mỗi app tự định nghĩa không gian project của mình.
type Options struct {
	// ValidProjectID báo một project ID có hợp lệ để gán quyền/member không.
	// nil → mọi ID được chấp nhận.
	ValidProjectID func(string) bool
	// LegacyProjectID là project mặc định cho resource ID không có phần
	// "<project>/" (vd. story ID trần từ thời chưa chia project).
	LegacyProjectID string
}

// validProjectID gọi validator của store; chưa cấu hình thì chấp nhận mọi ID.
func (o Options) validProjectID(id string) bool {
	return o.ValidProjectID == nil || o.ValidProjectID(id)
}
