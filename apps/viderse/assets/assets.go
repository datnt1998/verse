// Package assets nhúng tài nguyên tĩnh của Viderse (prompt hệ thống) vào
// binary — app không đọc file từ đĩa khi chạy.
package assets

import _ "embed"

//go:embed writer_system.md
var WriterSystemPrompt string
