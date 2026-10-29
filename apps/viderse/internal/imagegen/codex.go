package imagegen

// Bản chuyển của generateIsolatedImage + prepareIsolatedCall +
// verifyImageReadBoundary + requireImageReadiness + reconcileImageInvocation
// trong src/server/clone/images.ts (yt-factory).
//
// Mỗi lần vẽ = một invocation cô lập: CODEX_HOME mới chỉ chứa bản sao auth,
// refs dir riêng giữ ảnh tham chiếu, biên đọc được chứng minh bằng probe cơ
// học trước khi model được gọi. Không credential/config/log kế thừa nào rời
// invocation. Con trỏ phục hồi (work dir, owner, pid, session) ghi vào
// scratch của biên nhận để app chết giữa chừng vẫn đối chiếu được output
// thay vì vẽ lại mù quáng.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// codexScratch là con trỏ phục hồi của một invocation Codex — phần tương
// đương image-invocation journal của bản cũ, giờ nằm trong cột scratch của
// biên nhận Postgres.
type codexScratch struct {
	Owner        string       `json:"owner"`
	PID          int          `json:"pid"`
	Work         string       `json:"work"`
	Home         string       `json:"home,omitempty"`
	Target       string       `json:"target"`
	CodexVersion string       `json:"codexVersion"`
	SessionID    string       `json:"sessionId,omitempty"`
	Output       *ImageOutput `json:"output,omitempty"`
}

// ErrScratchInvalid: scratch không đọc/không khớp được — service rơi về kiểm
// tra output chung thay vì coi đây là bằng chứng "không cứu được".
var ErrScratchInvalid = errors.New("scratch biên nhận ảnh không đọc được")

// CodexBackend là đúng phần năng lực của tools.Codex mà imagegen dùng — một
// interface nhỏ phía consumer, không phải cả trình quản tool.
type CodexBackend interface {
	Status(ctx context.Context) tools.CodexStatus
	DefaultHome() string
	PrepareImageModelCatalog(ctx context.Context, exe, home string) (string, error)
}

// CodexDeps là phụ thuộc của CodexProvider.
type CodexDeps struct {
	// Codex: năng lực quản binary (Status, DefaultHome, PrepareImageModelCatalog).
	// Bắt buộc.
	Codex CodexBackend
	// Runner chạy mọi lời gọi codex (exec + probe cơ học); test tiêm bản giả.
	// Bắt buộc.
	Runner tools.Runner
	// StatusFunc ghi đè Codex.Status (test); nil → Codex.Status.
	StatusFunc func(ctx context.Context) (tools.CodexStatus, error)
	// SourceHome ghi đè Codex.DefaultHome() — nguồn auth.json.
	SourceHome string
	// PrepareCatalog ghi đè Codex.PrepareImageModelCatalog (test không cần
	// `debug models` thật); nil → bản thật.
	PrepareCatalog func(ctx context.Context, exe, home string) (string, error)
	// ForbiddenSentinels: cụm ký tự mà nếu lọt vào audit tail thì huỷ ngay
	// (biên bảo mật của bản cũ — secret canary của chủ sở hữu).
	ForbiddenSentinels []string
	// OwnPID hàm lấy pid tiến trình (test giả lập crash bằng pid chết).
	OwnPID func() int
}

// CodexProvider vẽ ảnh qua Codex CLI cô lập (model gpt-image-2).
type CodexProvider struct {
	d CodexDeps
}

// NewCodexProvider dựng provider Codex.
func NewCodexProvider(d CodexDeps) *CodexProvider {
	if d.OwnPID == nil {
		d.OwnPID = os.Getpid
	}
	return &CodexProvider{d: d}
}

func (p *CodexProvider) Name() string  { return "codex" }
func (p *CodexProvider) Model() string { return CodexImageModel }
func (p *CodexProvider) Paid() bool    { return true }

// status đọc trạng thái codex (StatusFunc khi test tiêm).
func (p *CodexProvider) status(ctx context.Context) (tools.CodexStatus, error) {
	if p.d.StatusFunc != nil {
		return p.d.StatusFunc(ctx)
	}
	if p.d.Codex == nil {
		return tools.CodexStatus{State: tools.CodexMissing}, nil
	}
	return p.d.Codex.Status(ctx), nil
}

// sourceHome trả home login của codex (auth.json nguồn).
func (p *CodexProvider) sourceHome() string {
	if p.d.SourceHome != "" {
		return p.d.SourceHome
	}
	if p.d.Codex != nil {
		return p.d.Codex.DefaultHome()
	}
	return ""
}

// Ready: codex sẵn sàng vẽ khi binary probe được, đăng nhập chatgpt và biên
// cô lập đã chứng minh (gồm allowlist image tool) — đúng requireImageReadiness.
func (p *CodexProvider) Ready(ctx context.Context) error {
	st, err := p.status(ctx)
	if err != nil {
		return err
	}
	return requireImageReadiness(st)
}

// requireImageReadiness ném tool_missing khi status chưa đủ chuẩn vẽ ảnh.
func requireImageReadiness(st tools.CodexStatus) error {
	ok := st.State == tools.CodexReady && st.Binary != nil && st.Login == tools.CodexLoginChatGPT &&
		st.Isolation != nil && st.Isolation.State == tools.IsolationVerified
	allowlist := false
	if ok {
		for _, c := range st.Isolation.Checks {
			if !c.OK {
				ok = false
				break
			}
			if c.Name == "image-tool-allowlist" {
				allowlist = true
			}
		}
	}
	if !ok || !allowlist {
		state := string(st.State)
		if st.State == tools.CodexReady {
			state = "isolation_unverified"
		}
		return MarkUnpaid(&DrawError{
			Code:     ErrCodeToolMissing,
			Message:  redactLogLine("Codex: " + state),
			Recovery: "Mở Cài đặt → Codex; đăng nhập ChatGPT và kiểm tra cách ly",
		})
	}
	return nil
}

// prepareIsolatedCall dựng môi trường cô lập của invocation: CODEX_HOME
// chỉ-auth + refs dir giữ bản sao ảnh tham chiếu, rồi chứng minh biên đọc
// bằng probe cơ học trước khi model được gọi.
func (p *CodexProvider) prepareIsolatedCall(ctx context.Context, exe, work, refs string, files []string, deny []string) (home, modelCatalog string, env map[string]string, attached []string, err error) {
	home, err = tools.CreateIsolatedCodexHome(p.sourceHome(), filepath.Join(work, "homes"))
	if err != nil {
		return "", "", nil, nil, err
	}
	if p.d.PrepareCatalog != nil {
		modelCatalog, err = p.d.PrepareCatalog(ctx, exe, home)
	} else {
		modelCatalog, err = p.d.Codex.PrepareImageModelCatalog(ctx, exe, home)
	}
	if err != nil {
		return "", "", nil, nil, err
	}
	for i, file := range files {
		ext := strings.ToLower(filepath.Ext(file))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
			return "", "", nil, nil, drawErr(ErrCodeInvalidRequest, "Định dạng ảnh tham chiếu không hỗ trợ", "")
		}
		copy := filepath.Join(refs, fmt.Sprintf("reference-%d%s", i+1, ext))
		if err := copyFileMode(file, copy); err != nil {
			return "", "", nil, nil, err
		}
		attached = append(attached, copy)
	}
	// Probe biên cần một tệp đọc được trong refs kể cả khi không đính kèm.
	if len(attached) == 0 {
		if err := os.WriteFile(filepath.Join(refs, "boundary-probe.txt"), []byte("readable"), 0o600); err != nil {
			return "", "", nil, nil, err
		}
	}
	env = map[string]string{
		"PATH":       filepath.Dir(exe) + ":/usr/bin:/bin",
		"HOME":       home,
		"CODEX_HOME": home,
		"LANG":       "en_US.UTF-8",
		"TMPDIR":     work,
	}
	secret := filepath.Join(work, ".env")
	if err := os.WriteFile(secret, []byte("IMAGE-READ-DENY-"+newUUID()), 0o600); err != nil {
		return "", "", nil, nil, err
	}
	if err := p.verifyReadBoundary(ctx, exe, refs, home, secret, env); err != nil {
		return "", "", nil, nil, err
	}
	for _, file := range deny {
		if err := p.verifyReadBoundary(ctx, exe, refs, home, file, env); err != nil {
			return "", "", nil, nil, err
		}
	}
	return home, modelCatalog, env, attached, nil
}

// copyFileMode sao chép file giữ quyền nguồn chặt (0600).
func copyFileMode(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

// verifyReadBoundary là probe cơ học: đọc được tệp đầu refs, không đọc được
// auth.json lẫn secret canary. Sai lệch nào cũng là isolation_unverified.
func (p *CodexProvider) verifyReadBoundary(ctx context.Context, exe, refs, home, secret string, env map[string]string) error {
	ents, err := os.ReadDir(refs)
	if err != nil {
		return err
	}
	if len(ents) == 0 {
		return errors.New("refs trống: không có tệp để probe")
	}
	type probe struct {
		file     string
		readable bool
	}
	probes := []probe{
		{filepath.Join(refs, ents[0].Name()), true},
		{filepath.Join(home, "auth.json"), false},
		{secret, false},
	}
	for _, pb := range probes {
		res, err := p.d.Runner.Run(ctx, exe, tools.SandboxProbeArgs(refs, []string{"/bin/sh", "-c", `exec 3<"$1"`, "read-probe", pb.file}), tools.Spec{
			Dir:      refs,
			EnvExact: env,
			Timeout:  30 * time.Second,
			OKCodes:  []int{0, 1, 2},
		})
		if err != nil {
			return err
		}
		if (res.Code == 0) != pb.readable {
			return &DrawError{
				Code:     ErrCodeToolMissing,
				Message:  "Codex: isolation_unverified (refs/auth/secret read boundary)",
				Recovery: "Khoá hàng đợi; kiểm tra cách ly",
			}
		}
	}
	return nil
}

// Draw là một lần vẽ ảnh trả phí qua Codex cô lập — điểm tiêu tiền duy nhất
// của provider này.
func (p *CodexProvider) Draw(ctx context.Context, req Request, journal JournalHandle) (Receipt, error) {
	st, err := p.status(ctx)
	if err != nil {
		return Receipt{}, MarkUnpaid(&DrawError{Code: ErrCodeToolMissing, Message: sanitizeProviderText(err.Error())})
	}
	if err := requireImageReadiness(st); err != nil {
		return Receipt{}, err
	}
	exe := st.Binary.Path
	owner := newUUID()

	work, err := os.MkdirTemp("", "ytf-image-call-")
	if err != nil {
		return Receipt{}, MarkUnpaid(err)
	}
	if err := os.Chmod(work, 0o700); err != nil {
		os.RemoveAll(work)
		return Receipt{}, MarkUnpaid(err)
	}
	defer os.RemoveAll(work)
	if err := os.WriteFile(filepath.Join(work, "ownership.json"), []byte(`{"owner":"`+owner+`"}`), 0o600); err != nil {
		return Receipt{}, MarkUnpaid(err)
	}
	refs := filepath.Join(work, "refs")
	if err := os.MkdirAll(refs, 0o700); err != nil {
		return Receipt{}, MarkUnpaid(err)
	}

	scratch := &codexScratch{
		Owner:        owner,
		PID:          p.d.OwnPID(),
		Work:         work,
		Target:       req.Output,
		CodexVersion: st.Binary.Version,
	}
	// Ghi con trỏ dọn dẹp TRƯỚC khi sao chép credential nào — đúng thứ tự của
	// journal file bản cũ. Lỗi ghi ở đây là lỗi store (chưa tiêu phí).
	if err := p.scratch(ctx, journal, scratch); err != nil {
		return Receipt{}, &StoreError{Err: err}
	}

	var files []string
	if req.Reference != "" {
		files = []string{req.Reference}
	}
	home, modelCatalog, env, attached, err := p.prepareIsolatedCall(ctx, exe, work, refs, files, req.Deny)
	if err != nil {
		return Receipt{}, MarkUnpaid(err)
	}
	scratch.Home = home

	parser := NewCodexImageOutputParser(home)
	journalState := ""
	// recordInvocation ghi scratch mới nhất (session/output tương quan) lên
	// biên nhận — bản cũ ghi lại journal file mỗi khi parser đổi trạng thái.
	recordInvocation := func() {
		var out *ImageOutput
		if len(parser.Outputs) == 1 && len(parser.ForbiddenTools) == 0 {
			o := parser.Outputs[0]
			o.Base64 = ""
			out = &o
		}
		scratch.SessionID = parser.SessionID
		scratch.Output = out
		data, _ := json.Marshal(scratch)
		if string(data) == journalState {
			return
		}
		journalState = string(data)
		// Lỗi ghi scratch không làm hỏng lần vẽ đang chạy.
		_ = journal.UpdateScratch(ctx, json.RawMessage(data))
	}
	recordInvocation()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var failed error
	var diagnostic string
	var auditTail string // chỉ giữ phần đuôi qua ranh giới dòng để bắt sentinel vắt dòng
	secretLeaked := false
	scanLine := func(line string, keepTail bool) {
		// Quét TOÀN BỘ dòng trước khi cắt đuôi: sentinel ở đầu một dòng dài
		// vẫn phải bị bắt (OnLine giao nguyên dòng, không phải chunk).
		window := auditTail + line
		for _, s := range p.d.ForbiddenSentinels {
			if s != "" && strings.Contains(window, s) {
				secretLeaked = true
				cancel()
			}
		}
		if keepTail {
			auditTail = tail(line, 16_384)
		} else {
			auditTail = ""
		}
	}
	args := tools.CodexExecArgs(refs, modelCatalog, attached, "", req.Prompt)
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	if timeout < 60*time.Second {
		timeout = 60 * time.Second
	}
	if timeout > 20*time.Minute {
		timeout = 20 * time.Minute
	}
	_, runErr := p.d.Runner.Run(runCtx, exe, args, tools.Spec{
		Dir:      refs,
		EnvExact: env,
		Timeout:  timeout,
		LogLimit: 8192,
		// Event ảnh base64 của một PNG 32 MiB ~44 MiB JSONL; 64 MiB chừa
		// envelope, khớp MaxImageBytes của parser.
		LineLimit: 64 << 20,
		OnLine: func(line string, stream tools.Stream) {
			scanLine(line, true)
			if stream == tools.StreamStderr {
				diagnostic = tail(diagnostic+line+"\n", 8192)
				return
			}
			if err := parser.Push(line + "\n"); err != nil {
				failed = err
				cancel()
				return
			}
			recordInvocation()
			if len(parser.ForbiddenTools) > 0 {
				cancel()
			}
		},
	})
	if runErr != nil && failed == nil {
		failed = runErr
	}
	if err := parser.Finish(); err != nil && failed == nil {
		failed = err
	}
	if len(parser.Outputs) == 0 {
		_ = ReadCurrentImageRollout(parser)
	}
	if secretLeaked {
		return Receipt{}, drawErr(ErrCodeToolMissing,
			"Codex: isolation_unverified (secret detected)", "Khoá hàng đợi; kiểm tra cách ly")
	}
	if len(parser.ForbiddenTools) > 0 {
		return Receipt{}, drawErr(ErrCodeToolMissing,
			"Codex: isolation_unverified (công cụ ngoài allowlist)",
			"Khoá tạo ảnh; kiểm tra cách ly trong Cài đặt")
	}
	if len(parser.Outputs) > 1 {
		return Receipt{}, drawErr(ErrCodeInternal, "Codex trả về nhiều ảnh cho một yêu cầu", "Kiểm tra adapter Codex")
	}
	owned, err := FindOwnedSessionImage(home, parser.SessionID)
	if err != nil {
		return Receipt{}, drawErr(ErrCodeInternal, redactLogLine(err.Error()), "Kiểm tra adapter Codex")
	}
	if owned != nil && len(parser.Outputs) > 0 && parser.Outputs[0].CallID != owned.CallID {
		return Receipt{}, drawErr(ErrCodeInternal, "Call ID của output không khớp phiên Codex", "")
	}
	if owned != nil && len(parser.Outputs) == 0 {
		parser.Outputs = append(parser.Outputs, *owned)
	}
	recordInvocation()

	// Output tương quan hợp lệ thắng kể cả khi huỷ tới sau khi lưu.
	if len(parser.Outputs) > 0 {
		out := parser.Outputs[0]
		if err := CaptureImage(out, home, req.Output); err != nil {
			return Receipt{}, drawErr(ErrCodeInternal, "Không lưu được ảnh: "+redactLogLine(err.Error()), "Tạo lại ảnh này")
		}
		sum, _ := sha256File(req.Output)
		receipt := Receipt{
			Provider: "codex", Model: CodexImageModel,
			SessionID: out.SessionID, CallID: out.CallID, Attribution: out.Attribution,
			CodexVersion: st.Binary.Version, PngSha256: sum,
		}
		if err := writeJSONAtomic(req.Output+".receipt.json", receipt); err != nil {
			// Ảnh đã nguyên tử trên đĩa; sidecar lỗi không được xoá ảnh và
			// không được dẫn tới vẽ lại.
			return Receipt{}, &StoreError{Err: err}
		}
		return receipt, nil
	}
	if ctx.Err() != nil {
		return Receipt{}, cancelledErr()
	}
	var text string
	var pfe *tools.ProcessFailedError
	switch {
	case parser.ErrorText != "":
		text = parser.ErrorText
	case parser.FinalSummary != "":
		text = parser.FinalSummary
	case errors.As(failed, &pfe):
		text = pfe.Stderr + pfe.Stdout
	case diagnostic != "":
		text = diagnostic
	case failed != nil:
		text = failed.Error()
	default:
		text = "no image output"
	}
	// Mọi văn bản provider trước khi vào lỗi/biên nhận đi qua sanitizer.
	text = sanitizeProviderText(text)
	if errors.As(failed, &pfe) && strings.Contains(diagnostic, "Error:") {
		startup := startupErrorLine(diagnostic)
		if startupBadRe.MatchString(startup) {
			msg := sanitizeProviderText(startup)
			return Receipt{}, drawErr(ErrCodeToolMissing,
				"Codex startup: "+msg, "Kiểm tra binary và cách ly Codex trong Cài đặt")
		}
	}
	if errors.Is(failed, tools.ErrCancelled) {
		return Receipt{}, ClassifyImageFailure("timeout", nil)
	}
	var retryAt *time.Time
	if parser.RetryAt != nil {
		t := time.UnixMilli(*parser.RetryAt)
		retryAt = &t
	}
	classified := ClassifyImageFailure(text, retryAt)
	if IsImageRefused(classified) || IsProviderBusy(classified) {
		return Receipt{}, classified
	}
	if de, ok := drawErrorOf(classified); ok && de.Code == ErrCodeQuota {
		return Receipt{}, classified
	}
	if de, ok := drawErrorOf(classified); ok && de.Code == ErrCodeToolMissing {
		return Receipt{}, classified
	}
	if transientRe.MatchString(text) {
		// Lỗi transport bất định của Codex: giữ retry (hành vi đã cam kết) —
		// Reconcile theo session/scratch sẽ cứu ảnh nếu nó đã được lưu.
		return Receipt{}, &tools.ProcessFailedError{
			Message: "Codex transport unavailable", Code: 1, Stderr: "temporarily unavailable",
		}
	}
	if parser.FinalSummary != "" {
		return Receipt{}, &DrawError{Code: ErrCodeInternal,
			Message:  "Không có ảnh: " + head(sanitizeProviderText(parser.FinalSummary), 500),
			Recovery: "Tạo lại ảnh này; không có ảnh thay thế giả"}
	}
	return Receipt{}, &DrawError{Code: ErrCodeInternal,
		Message:  "Codex không trả về ảnh hợp lệ: " + tail(text, 400),
		Recovery: "Tạo lại ảnh này; không có ảnh thay thế giả"}
}

// Reconcile áp luật reconcileImageInvocation của bản cũ lên scratch đã ghi:
// chỉ đối chiếu đúng invocation sở hữu, không bao giờ quét thư mục đích.
func (p *CodexProvider) Reconcile(ctx context.Context, raw json.RawMessage, req Request) (Receipt, bool, error) {
	var sc codexScratch
	if err := json.Unmarshal(raw, &sc); err != nil || sc.Owner == "" || sc.PID <= 0 || sc.Work == "" {
		return Receipt{}, false, ErrScratchInvalid
	}
	targetAbs, _ := filepath.Abs(req.Output)
	scratchAbs, _ := filepath.Abs(sc.Target)
	if scratchAbs != targetAbs {
		return Receipt{}, false, drawErr(ErrCodeInvalidRequest, "Phiên tạo ảnh không khớp output", "")
	}
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		tmp = os.TempDir()
	}
	work, err := filepath.EvalSymlinks(sc.Work)
	if err != nil {
		return Receipt{}, false, nil // work đã dọn: rơi về kiểm tra output
	}
	home := ""
	if sc.Home != "" {
		if h, err := filepath.EvalSymlinks(sc.Home); err == nil {
			home = h
		}
	}
	if !strings.HasPrefix(work, filepath.Join(tmp, "ytf-image-call-")) ||
		(home != "" && !strings.HasPrefix(home, filepath.Join(work, "homes")+string(os.PathSeparator))) {
		return Receipt{}, false, drawErr(ErrCodeInvalidRequest, "Phiên tạo ảnh không thuộc app", "")
	}
	st, err := os.Stat(work)
	if err != nil || st.Mode().Perm() != 0o700 {
		return Receipt{}, false, drawErr(ErrCodeInvalidRequest, "Phiên tạo ảnh không thuộc app", "")
	}
	ownerData, err := os.ReadFile(filepath.Join(work, "ownership.json"))
	if err != nil {
		return Receipt{}, false, drawErr(ErrCodeInvalidRequest, "Phiên tạo ảnh không thuộc app", "")
	}
	var ownership struct {
		Owner string `json:"owner"`
	}
	if err := json.Unmarshal(ownerData, &ownership); err != nil || ownership.Owner != sc.Owner {
		return Receipt{}, false, drawErr(ErrCodeInvalidRequest, "Phiên tạo ảnh không thuộc app", "")
	}
	if pidAlive(sc.PID) {
		return Receipt{}, false, drawErr(ErrCodeConflict, "Phiên tạo ảnh vẫn đang chạy", "")
	}
	// Phiên đã chết: dọn work của invocation sau khi đối chiếu xong — đúng
	// tinh thần cleanupStaleImageWork của bản cũ (xong-kết-luận mới xoá).
	defer os.RemoveAll(sc.Work)

	out := sc.Output
	if out == nil && home != "" {
		if owned, err := FindOwnedSessionImage(home, sc.SessionID); err == nil {
			out = owned
		}
	}
	if out == nil || out.SavedPath == "" || home == "" {
		return Receipt{}, false, nil
	}
	if _, err := os.Stat(req.Output); err == nil {
		// Ngắt giữa copy và receipt: chỉ khi ảnh đích trùng byte với bản gốc
		// được gán mới được cứu — không giả định về một PNG bất kỳ.
		check := req.Output + ".reconcile"
		defer os.Remove(check)
		if err := CaptureImage(*out, home, check); err != nil {
			return Receipt{}, false, nil
		}
		sumCheck, _ := sha256File(check)
		sumTarget, _ := sha256File(req.Output)
		if sumCheck == "" || sumCheck != sumTarget {
			return Receipt{}, false, drawErr(ErrCodeStale, "PNG đã lưu không khớp phiên tạo ảnh", "")
		}
	} else if err := CaptureImage(*out, home, req.Output); err != nil {
		return Receipt{}, false, nil
	}
	sum, _ := sha256File(req.Output)
	receipt := Receipt{
		Provider: "codex", Model: CodexImageModel,
		SessionID: out.SessionID, CallID: out.CallID, Attribution: out.Attribution,
		CodexVersion: sc.CodexVersion, PngSha256: sum,
	}
	_ = writeJSONAtomic(req.Output+".receipt.json", receipt)
	return receipt, true, nil
}

// scratch ghi con trỏ phục hồi lên biên nhận.
func (p *CodexProvider) scratch(ctx context.Context, journal JournalHandle, sc *codexScratch) error {
	data, err := json.Marshal(sc)
	if err != nil {
		return err
	}
	return journal.UpdateScratch(ctx, json.RawMessage(data))
}

// startupErrorLine rút dòng `Error: …` đầu của diagnostic.
func startupErrorLine(diagnostic string) string {
	idx := strings.Index(diagnostic, "Error: ")
	if idx < 0 {
		return ""
	}
	rest := diagnostic[idx+7:]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[:nl]
	}
	return head(rest, 500)
}

// newUUID trả UUID v4 ngẫu nhiên (không kéo thêm dep).
func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
