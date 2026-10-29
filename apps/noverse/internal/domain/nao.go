package domain

// RelationshipChange là một dữ kiện quan hệ có nguồn; From/To dùng mã cục bộ.
// Reason chỉ có khi người dùng sửa trên web (GAP-38); engine ghi để trống.
// Type là kiểu quan hệ cho đồ thị Não truyện (D10); dữ kiện cũ không có → "rel".
type RelationshipChange struct {
	Chapter  int    `json:"chapter"`
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"`
	Type     string `json:"type,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Scope    string `json:"scope"`
	Source   string `json:"source"`
}

// RelationshipTypes là sáu kiểu quan hệ của Não truyện (22-brief-2510 D10).
var relationshipTypes = []string{"enemy", "ally", "work", "own", "secret", "rel"}

// RelationshipTypes trả bản sao danh sách kiểu quan hệ cho schema công cụ.
func RelationshipTypes() []string {
	return append([]string(nil), relationshipTypes...)
}

// ValidRelationshipType báo giá trị có nằm trong enum kiểu quan hệ hay không.
func ValidRelationshipType(t string) bool {
	for _, known := range relationshipTypes {
		if t == known {
			return true
		}
	}
	return false
}

// NormalizeRelationshipType quy dữ kiện thiếu kiểu (bản ghi cũ, model trả null) về "rel".
func NormalizeRelationshipType(t string) string {
	if ValidRelationshipType(t) {
		return t
	}
	return "rel"
}

// AddressTermInput nhận tên hoặc biệt danh ở công cụ, mã nhân vật ở kho dữ liệu.
type AddressTermInput struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Calls  string `json:"calls"`
	Self   string `json:"self"`
	Reason string `json:"reason"`
}

// AddressTerm có chiều: From gọi To; UntilChapter = 0 nghĩa là còn hiệu lực.
type AddressTerm struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Calls        string `json:"calls"`
	Self         string `json:"self"`
	FromChapter  int    `json:"from_chapter"`
	UntilChapter int    `json:"until_chapter"`
	Reason       string `json:"reason"`
	Scope        string `json:"scope"`
	Source       string `json:"source"`
}

// QualifiedCharacterID tính phạm vi từ vị trí hiện tại, không ghi vào characters.json.
func QualifiedCharacterID(project, story, localID string) string {
	return "truyen:" + project + "/" + story + ":" + localID
}
