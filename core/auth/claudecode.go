package auth

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
)

// Dấu vân tay Claude Code (port từ omp: ai/src/providers/claude-code-fingerprint.ts).
// Gói thuê bao Claude chỉ chấp nhận request trông như đến từ Claude Code CLI, nên
// version/User-Agent phải khớp một bản phát hành thật.
const (
	// DefaultClaudeCodeVersion là bản Claude Code CLI được ghim làm mặc định.
	DefaultClaudeCodeVersion = "2.1.280"
	// ClaudeCodeSDKVersion là phiên bản @anthropic-ai/sdk đi kèm bản Claude Code trên.
	ClaudeCodeSDKVersion = "0.112.1"
	// ClaudeCodeIdentity là khối system đầu tiên mà Claude Code CLI luôn gửi.
	ClaudeCodeIdentity = "You are Claude Code, Anthropic's official CLI for Claude."
	// ClaudeToolPrefix tách tool tuỳ biến khỏi tool dựng sẵn của Claude Code (chỉ trên wire).
	ClaudeToolPrefix = "_"
)

var (
	semverPattern          = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)
	requiredVersionPattern = regexp.MustCompile(`(?i)version (\d+\.\d+\.\d+) or newer is required`)
	adoptedVersion         atomic.Pointer[string]
)

// ClaudeCodeVersion là version Claude Code đại diện trên wire (User-Agent, billing header):
// biến môi trường → version học từ lần bị từ chối → mặc định ghim.
func ClaudeCodeVersion() string {
	if env := options().ClaudeCodeVersionEnv; env != "" {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	if v := adoptedVersion.Load(); v != nil {
		return *v
	}
	return DefaultClaudeCodeVersion
}

// ClaudeCodeUserAgent là User-Agent của entrypoint inference của Claude Code CLI.
func ClaudeCodeUserAgent() string {
	return fmt.Sprintf("claude-cli/%s (external, cli)", ClaudeCodeVersion())
}

// AdoptRequiredClaudeCodeVersion nhận version tối thiểu từ lỗi `claude_code_version_too_old`
// của Anthropic cho phần còn lại của tiến trình, để bản ghim lỗi thời chỉ tốn một request
// bị từ chối thay vì hỏng hẳn. Chỉ trả true khi version thật sự tăng (nơi gọi thử lại khi
// true nên không thể lặp vô hạn); luôn false khi biến môi trường đã ghim version.
func AdoptRequiredClaudeCodeVersion(message string) bool {
	if env := options().ClaudeCodeVersionEnv; env != "" && strings.TrimSpace(os.Getenv(env)) != "" {
		return false
	}
	if !strings.Contains(message, "claude_code_version_too_old") {
		return false
	}
	m := requiredVersionPattern.FindStringSubmatch(message)
	if m == nil || compareSemver(m[1], ClaudeCodeVersion()) <= 0 {
		return false
	}
	v := m[1]
	adoptedVersion.Store(&v)
	return true
}

// compareSemver so sánh hai x.y.z; chuỗi sai dạng coi như bằng nhau.
func compareSemver(a, b string) int {
	pa, pb := semverPattern.FindStringSubmatch(a), semverPattern.FindStringSubmatch(b)
	if pa == nil || pb == nil {
		return 0
	}
	for i := 1; i <= 3; i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x - y
		}
	}
	return 0
}
