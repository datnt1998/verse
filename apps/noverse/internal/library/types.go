package library

import (
	"errors"
	"time"
)

// 书库操作的哨兵错误；gateway 按类型映射为 400/404/409。
var (
	ErrInvalidName  = errors.New("tên không hợp lệ")
	ErrExists       = errors.New("đã tồn tại")
	ErrNotFound     = errors.New("không tìm thấy")
	ErrNotCandidate = errors.New("thư mục không phải ứng viên dự án")
	ErrIdeaNotFound = errors.New("không tìm thấy ý tưởng")
	ErrLegacy       = errors.New("dự án ẩn danh không hỗ trợ thao tác này")
)

// StoryStatus 是 story.json 与 API 共用的一组状态。
var StoryStatus = map[string]bool{
	"idea": true, "writing": true, "paused": true, "done": true, "published": true,
}

// Idea 对应 <project>/<dirs.ideas>/<id>.md。
type Idea struct {
	ID        string     `json:"id"` // 文件名（不含 .md）
	Title     string     `json:"title"`
	Excerpt   string     `json:"excerpt"`
	CreatedAt *time.Time `json:"createdAt"`
	UpdatedAt *time.Time `json:"updatedAt"`
	UsedBy    []string   `json:"usedBy"` // story.json.idea == id 的故事 id
}

// Asset 是 <project>/<dirs.assets> 下的一个条目。
type Asset struct {
	Path      string     `json:"path"` // 相对资产目录，"/" 分隔
	Kind      string     `json:"kind"` // file | dir
	Size      int64      `json:"size"`
	UpdatedAt *time.Time `json:"updatedAt"`
}

// FolderNode 描述项目目录树的期望结构与实际状态，供 validate 接口渲染。
type FolderNode struct {
	Name     string        `json:"name"`
	Path     string        `json:"path"`   // 相对项目目录
	Kind     string        `json:"kind"`   // file | dir
	Role     string        `json:"role"`   // project|config|rules|ideas|assets|stories|story|storyMeta|engine|exports|other
	Status   string        `json:"status"` // ok|engine|optional|missing|invalid
	Note     string        `json:"note,omitempty"`
	Children []*FolderNode `json:"children,omitempty"`
}

// ValidationIssue 是目录检查发现的一个问题；Fixable 表示 Fix 可以安全处理。
type ValidationIssue struct {
	Path    string `json:"path"` // 相对项目目录
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Fixable bool   `json:"fixable"`
}

// Validation 是 validate / validate/fix 的返回体。
type Validation struct {
	OK     bool              `json:"ok"`
	Issues []ValidationIssue `json:"issues"`
	Tree   *FolderNode       `json:"tree"`
}
