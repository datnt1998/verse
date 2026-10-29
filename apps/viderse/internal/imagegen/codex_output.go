package imagegen

// Bản chuyển của src/server/clone/codex-image-output.ts (yt-factory): kiểm
// PNG, parser JSONL của `codex exec --json`, và các luật đối chiếu output —
// không bao giờ theo symlink hay "file mới nhất", chỉ session/call đã định
// danh của chính invocation.

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// MaxImageBytes là trần kích thước ảnh provider trả về (32 MB như bản cũ).
const MaxImageBytes = 32 * 1024 * 1024

const maxImageLine = (MaxImageBytes/3+1)*4 + 1024*1024

var (
	idRe          = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	callFileRe    = regexp.MustCompile(`^call_[A-Za-z0-9_-]+\.png$`)
	allowedCallRe = regexp.MustCompile(`^(image_gen(?:\.|__)imagegen|functions\.view_image|view_image)$`)
)

var pngMagic = []byte{137, 80, 78, 71, 13, 10, 26, 10}

// ImageOutput là một output ảnh được định danh của phiên Codex.
type ImageOutput struct {
	SessionID   string `json:"sessionId"`
	CallID      string `json:"callId"`
	SavedPath   string `json:"savedPath,omitempty"`
	Base64      string `json:"base64,omitempty"`
	Model       string `json:"model,omitempty"`
	Attribution string `json:"attribution"` // event | reported-path | rollout | owned-session-file
}

// validatePngShape kiểm cặp đầu/đuôi PNG: magic, IHDR, kích thước, IEND.
func validatePngShape(head, tail []byte, size int64) error {
	var width, height uint32
	if len(head) >= 24 {
		width = binary.BigEndian.Uint32(head[16:20])
		height = binary.BigEndian.Uint32(head[20:24])
	}
	if size < 45 || size > MaxImageBytes || len(head) < 24 || len(tail) < 8 ||
		!bytes.Equal(head[:8], pngMagic) || string(head[12:16]) != "IHDR" ||
		width == 0 || height == 0 || width > 8192 || height > 8192 ||
		int64(width)*int64(height) > 32_000_000 || string(tail[:4]) != "IEND" {
		return errors.New("invalid_image: PNG signature, dimensions, terminator or size invalid")
	}
	return nil
}

// ValidatePng kiểm một payload PNG trong bộ nhớ.
func ValidatePng(b []byte) error {
	head := b
	if len(head) > 24 {
		head = head[:24]
	}
	tail := b
	if len(tail) > 8 {
		tail = tail[len(tail)-8:]
	}
	return validatePngShape(head, tail, int64(len(b)))
}

// ValidatePngFile kiểm tệp PNG (mở với O_NOFOLLOW, đọc đầu/đuôi).
func ValidatePngFile(file string) error {
	f, err := openNoFollow(file)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Size() < 45 || st.Size() > MaxImageBytes {
		return errors.New("invalid_image: PNG file size/type invalid")
	}
	head := make([]byte, 24)
	tail := make([]byte, 8)
	if _, err := f.ReadAt(head, 0); err != nil {
		return err
	}
	if _, err := f.ReadAt(tail, st.Size()-int64(len(tail))); err != nil {
		return err
	}
	return validatePngShape(head, tail, st.Size())
}

// DecodeImageBase64 giải mã base64 canonical thành PNG hợp lệ.
func DecodeImageBase64(value string) ([]byte, error) {
	if len(value) > maxImageLine || len(value)%4 != 0 {
		return nil, errors.New("invalid_image: invalid base64")
	}
	for _, c := range []byte(value) {
		ok := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/' || c == '='
		if !ok {
			return nil, errors.New("invalid_image: invalid base64")
		}
	}
	if strings.Count(value, "=") > 2 || (strings.Contains(value, "=") && !strings.HasSuffix(value, strings.Repeat("=", strings.Count(value, "="))) || strings.Index(value, "=") < len(value)-2) {
		return nil, errors.New("invalid_image: invalid base64")
	}
	bytes_, err := base64.StdEncoding.DecodeString(value)
	if err != nil || base64.StdEncoding.EncodeToString(bytes_) != value {
		return nil, errors.New("invalid_image: noncanonical base64")
	}
	if err := ValidatePng(bytes_); err != nil {
		return nil, err
	}
	return bytes_, nil
}

// CodexImageOutputParser đọc stream JSONL của `codex exec --json`. Một
// instance cho một invocation; prose thô không được giữ lại.
type CodexImageOutputParser struct {
	home string

	pending       []string
	pendingLength int

	SessionID      string
	Outputs        []ImageOutput
	ForbiddenTools []string
	ErrorText      string
	FinalSummary   string
	RetryAt        *int64 // ms epoch do provider báo (failure.resets_at)

	allowedCall *regexp.Regexp // nil = cấm mọi call (chế độ text)
}

// NewCodexImageOutputParser dựng parser cho CODEX_HOME của invocation.
func NewCodexImageOutputParser(home string) *CodexImageOutputParser {
	return &CodexImageOutputParser{home: home, allowedCall: allowedCallRe}
}

// Push nạp một chunk stdout; chỉ gọi line() trên dòng hoàn chỉnh.
func (p *CodexImageOutputParser) Push(chunk string) error {
	start := 0
	for {
		idx := strings.Index(chunk[start:], "\n")
		if idx < 0 {
			break
		}
		part := chunk[start : start+idx]
		if p.pendingLength+len(part) > maxImageLine {
			return errors.New("image_output_too_large")
		}
		if len(p.pending) > 0 {
			p.pending = append(p.pending, part)
			if err := p.Line(strings.Join(p.pending, ""), "event"); err != nil {
				return err
			}
			p.pending = nil
			p.pendingLength = 0
		} else if err := p.Line(part, "event"); err != nil {
			return err
		}
		start += idx + 1
	}
	if start < len(chunk) {
		part := chunk[start:]
		p.pendingLength += len(part)
		if p.pendingLength > maxImageLine || len(p.pending) >= 8192 {
			return errors.New("image_output_too_large")
		}
		p.pending = append(p.pending, part)
	}
	return nil
}

// Finish xử lý phần dư cuối stream.
func (p *CodexImageOutputParser) Finish() error {
	if len(p.pending) > 0 {
		if err := p.Line(strings.Join(p.pending, ""), "event"); err != nil {
			return err
		}
	}
	p.pending = nil
	p.pendingLength = 0
	return nil
}

// Line xử lý một dòng JSONL. Non-JSON bị bỏ qua (log của CLI).
func (p *CodexImageOutputParser) Line(line, attribution string) error {
	if len(line) > maxImageLine {
		return errors.New("image_output_too_large")
	}
	var e map[string]any
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		return nil
	}
	if str(e["type"]) == "thread.started" {
		if id, ok := e["thread_id"].(string); ok {
			if !idRe.MatchString(id) || (p.SessionID != "" && p.SessionID != id) {
				return errors.New("image_session_mismatch")
			}
			p.SessionID = id
		}
	}
	item, _ := e["item"].(map[string]any)
	if item == nil {
		item, _ = e["payload"].(map[string]any)
	}
	if item == nil {
		item = e
	}
	typ := str(item["type"])
	switch typ {
	case "command_execution", "file_change", "mcp_tool_call", "collab_tool_call", "web_search", "custom_tool_call":
		p.ForbiddenTools = append(p.ForbiddenTools, typ)
	}
	if typ == "function_call" {
		name := str(item["name"])
		if item["namespace"] != nil {
			name = str(item["namespace"]) + "." + name
		}
		if p.allowedCall == nil || !p.allowedCall.MatchString(name) {
			p.ForbiddenTools = append(p.ForbiddenTools, name)
		}
	}
	if str(e["type"]) == "error" || str(e["type"]) == "turn.failed" || item["failure"] != nil ||
		(typ == "error" && imageQuotaRe.MatchString(str(item["message"]))) {
		data, _ := json.Marshal(e)
		p.ErrorText = head(string(data), 8000)
		if failure, ok := item["failure"].(map[string]any); ok {
			if resets, ok := failure["resets_at"].(float64); ok && resets > 0 {
				ms := int64(resets * 1000)
				p.RetryAt = &ms
			}
		}
	}
	if p.SessionID == "" {
		return nil
	}
	if typ == "image_generation" || typ == "image_generation_call" || typ == "image_generation_end" {
		callID := str(item["call_id"])
		if callID == "" {
			callID = str(item["id"])
		}
		if !idRe.MatchString(callID) {
			return nil
		}
		if st := str(item["status"]); st != "" && st != "completed" && st != "generating" {
			return nil
		}
		out := ImageOutput{SessionID: p.SessionID, CallID: callID, Attribution: attribution}
		if v, ok := item["saved_path"].(string); ok {
			out.SavedPath = v
		}
		if v, ok := item["result"].(string); ok && v != "" {
			out.Base64 = v
		}
		if v, ok := item["model"].(string); ok {
			out.Model = v
		}
		if out.SavedPath != "" || out.Base64 != "" {
			p.add(out)
		}
	}
	// 0.162.1 exec bỏ sót item ảnh; gợi ý đường dẫn chính xác của tool nằm
	// trong message cuối — cả thư mục lẫn tên file đều tương quan với phiên.
	if typ == "agent_message" {
		if text, ok := item["text"].(string); ok {
			p.FinalSummary = head(text, 1000)
			homes := map[string]bool{p.home: true}
			if real, err := filepath.EvalSymlinks(p.home); err == nil {
				homes[real] = true
			}
			for home := range homes {
				root := filepath.Join(home, "generated_images", p.SessionID) + string(os.PathSeparator)
				re := regexp.MustCompile(regexp.QuoteMeta(root) + `([A-Za-z0-9_-]+)\.png`)
				for _, m := range re.FindAllStringSubmatch(text, -1) {
					p.add(ImageOutput{SessionID: p.SessionID, CallID: m[1], SavedPath: m[0], Attribution: "reported-path"})
				}
			}
			if imageQuotaRe.MatchString(text) {
				p.ErrorText = head(text, 8000)
			}
		}
	}
	return nil
}

func (p *CodexImageOutputParser) add(out ImageOutput) {
	for i, o := range p.Outputs {
		if o.CallID == out.CallID {
			p.Outputs[i] = out
			return
		}
	}
	p.Outputs = append(p.Outputs, out)
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ReadImageRollout đọc CHỈ rollout đã tương quan của phiên trong home này.
func ReadImageRollout(file string, p *CodexImageOutputParser) error {
	home, err := filepath.EvalSymlinks(p.home)
	if err != nil {
		return err
	}
	real, err := filepath.EvalSymlinks(file)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(real, filepath.Join(home, "sessions")+string(os.PathSeparator)) ||
		p.SessionID == "" || !strings.Contains(filepath.Base(real), p.SessionID) {
		return errors.New("image_rollout_scope")
	}
	if st, err := os.Lstat(file); err != nil || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("image_rollout_scope")
	}
	if st, err := os.Stat(real); err != nil || st.Size() > 64*1024*1024 {
		return errors.New("image_rollout_too_large")
	}
	f, err := os.Open(real)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), maxImageLine+1024)
	for sc.Scan() {
		if err := p.Line(sc.Text(), "rollout"); err != nil {
			return err
		}
	}
	return sc.Err()
}

// ReadCurrentImageRollout tìm rollout duy nhất của phiên trong home của
// invocation (ephemeral thường không có; không bao giờ mò sang home của chủ).
func ReadCurrentImageRollout(p *CodexImageOutputParser) error {
	if p.SessionID == "" {
		return nil
	}
	root := filepath.Join(p.home, "sessions")
	if _, err := os.Stat(root); err != nil {
		return nil
	}
	type entry struct {
		dir   string
		depth int
	}
	queue := []entry{{root, 0}}
	count := 0
	var matches []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		ents, err := os.ReadDir(cur.dir)
		if err != nil {
			return err
		}
		for _, en := range ents {
			count++
			if count > 1000 {
				return errors.New("image_rollout_too_many_files")
			}
			if en.Type()&os.ModeSymlink != 0 {
				return errors.New("image_rollout_scope")
			}
			file := filepath.Join(cur.dir, en.Name())
			if en.IsDir() && cur.depth < 4 {
				queue = append(queue, entry{file, cur.depth + 1})
			} else if en.Type().IsRegular() && strings.HasSuffix(en.Name(), p.SessionID+".jsonl") {
				matches = append(matches, file)
			}
		}
	}
	sort.Strings(matches)
	if len(matches) > 1 {
		return errors.New("image_rollout_ambiguous")
	}
	if len(matches) == 1 {
		return ReadImageRollout(matches[0], p)
	}
	return nil
}

// FindOwnedSessionImage: 0.162.1 có thể lưu trước khi phát message cuối.
// Đối chiếu filesystem duy nhất được phép là phiên đã định danh của invocation
// này với đúng một file output theo call-ID. Nhiều file là mơ hồ — không bao
// giờ chọn "mới nhất".
func FindOwnedSessionImage(home, sessionID string) (*ImageOutput, error) {
	if sessionID == "" || !idRe.MatchString(sessionID) {
		return nil, nil
	}
	dir := filepath.Join(home, "generated_images", sessionID)
	if _, err := os.Stat(dir); err != nil {
		return nil, nil
	}
	expected, err := filepath.EvalSymlinks(filepath.Join(home, "generated_images", sessionID))
	if err != nil {
		return nil, err
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil || realDir != expected {
		return nil, errors.New("image_path_scope")
	}
	if st, err := os.Lstat(dir); err != nil || st.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("image_path_scope")
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	if len(ents) == 0 {
		return nil, nil
	}
	if len(ents) != 1 {
		return nil, errors.New("image_output_ambiguous")
	}
	en := ents[0]
	if !en.Type().IsRegular() || !callFileRe.MatchString(en.Name()) {
		return nil, errors.New("image_attribution_invalid")
	}
	savedPath := filepath.Join(dir, en.Name())
	if err := ValidatePngFile(savedPath); err != nil {
		return nil, err
	}
	return &ImageOutput{
		SessionID:   sessionID,
		CallID:      strings.TrimSuffix(en.Name(), ".png"),
		SavedPath:   savedPath,
		Attribution: "owned-session-file",
	}, nil
}

// CaptureImage chép output về đích TRƯỚC khi huỷ auth/home; không theo
// symlink, không quét newest.
func CaptureImage(out ImageOutput, home, target string) error {
	if !idRe.MatchString(out.SessionID) || !idRe.MatchString(out.CallID) {
		return errors.New("image_attribution_invalid")
	}
	if out.Model != "" && out.Model != CodexImageModel {
		return errors.New("image_model_mismatch")
	}
	if out.SavedPath != "" {
		realHome, err := filepath.EvalSymlinks(home)
		if err != nil {
			return err
		}
		expected := filepath.Join(realHome, "generated_images", out.SessionID, out.CallID+".png")
		actual, err := filepath.Abs(out.SavedPath)
		if err != nil {
			return err
		}
		lexical := filepath.Join(home, "generated_images", out.SessionID, out.CallID+".png")
		if actual != lexical && actual != expected {
			return errors.New("image_path_scope")
		}
		realActual, err := filepath.EvalSymlinks(actual)
		if err != nil || realActual != expected {
			return errors.New("image_path_scope")
		}
		if st, err := os.Lstat(actual); err != nil || !st.Mode().IsRegular() {
			return errors.New("image_path_scope")
		}
		if err := ValidatePngFile(actual); err != nil {
			return err
		}
		return copyAtomicExclusive(actual, target, 0o600)
	}
	if out.Base64 != "" {
		data, err := DecodeImageBase64(out.Base64)
		if err != nil {
			return err
		}
		return writeAtomicExclusive(target, data, 0o600)
	}
	return errors.New("image_output_missing")
}
