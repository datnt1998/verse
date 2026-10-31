package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

const cloneUsage = `Cách dùng:
  viderse clone run <url> --channel <id> [--priority pick|batch] [--no-images]
  viderse clone run <jobId> [--wait] [--timeout GIÂY]
  viderse clone batch <url>... --channel <id> [--priority pick|batch] [--no-images]
  viderse clone pause|resume|cancel <jobId>
  viderse clone status [jobId] [--channel <id>] [--limit N] [--counts]
  viderse clone status <jobId> --watch [--timeout GIÂY]
  viderse clone wait <jobId> [--timeout GIÂY]

  --for <id> là cách viết khác của --channel khi xếp video vào hàng chờ.
  run URL mặc định ưu tiên pick; batch mặc định batch. Mọi URL đều vào hàng chờ.
  wait dừng khi xong, thất bại, bị huỷ hoặc chờ duyệt; thất bại trả exit 1.
  --timeout mặc định 1800 giây. Tạm dừng hoặc chờ hạn mức không được coi là xong.

Kết nối máy chủ đã chạy:
  --api URL       PUBLIC_URL, hoặc http://VIDERSE_ADDR (mặc định 127.0.0.1:8789)
  --cookie CHUỖI  Cookie phiên đăng nhập, mặc định VIDERSE_COOKIE (verse_session=…)
JSON ra stdout; lỗi JSON ra stderr. Không tự khởi động worker hay bỏ qua đăng nhập.
`

type cloneOptions struct {
	command  string
	operands []string
	channel  string
	priority domain.ClonePriority
	noImages bool
	wait     bool
	counts   bool
	limit    int
	timeout  time.Duration
	api      string
	cookie   string
}

type cloneCLIError struct {
	Message  string          `json:"error"`
	Code     string          `json:"code,omitempty"`
	Recovery *string         `json:"recovery,omitempty"`
	RetryAt  *time.Time      `json:"retryAt,omitempty"`
	Details  json.RawMessage `json:"details,omitempty"`
}

func (e *cloneCLIError) Error() string { return e.Message }

func cloneInvalid(message string) error {
	return &cloneCLIError{Message: message, Code: domain.CloneErrInvalidRequest}
}

func runCloneCommand(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runCloneCommandContext(ctx, args, stdout, stderr)
}

func runCloneCommandContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		if _, err := io.WriteString(stdout, cloneUsage); err != nil {
			return cloneErrOut(stderr, err)
		}
		return 0
	}
	options, err := parseCloneOptions(args, os.Getenv)
	if err != nil {
		return cloneErrOut(stderr, err)
	}
	client := cloneHTTPClient{
		base: options.api, cookie: options.cookie, pollInterval: time.Second,
		http: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	result, err := client.execute(ctx, options)
	if err != nil {
		return cloneErrOut(stderr, err)
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return cloneErrOut(stderr, fmt.Errorf("không ghi được kết quả: %w", err))
	}
	return 0
}

func parseCloneOptions(args []string, getenv func(string) string) (cloneOptions, error) {
	o := cloneOptions{limit: 50, timeout: 30 * time.Minute}
	if len(args) == 0 {
		return o, cloneInvalid("thiếu lệnh con (run|batch|pause|resume|cancel|status|wait)")
	}
	o.command = args[0]
	switch o.command {
	case "run", "batch", "pause", "resume", "cancel", "status", "wait":
	default:
		return o, cloneInvalid("lệnh con không hợp lệ: " + o.command)
	}
	seen := make(map[string]bool)
	for i := 1; i < len(args); i++ {
		raw := args[i]
		if raw == "--" {
			o.operands = append(o.operands, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(raw, "-") {
			o.operands = append(o.operands, raw)
			continue
		}
		name, value, hasValue := strings.Cut(raw, "=")
		if name == "--for" {
			name = "--channel"
		}
		if name == "--watch" {
			if o.command != "status" {
				return o, cloneInvalid("--watch chỉ dùng với status")
			}
			name = "--wait"
		}
		if seen[name] {
			return o, cloneInvalid("cờ bị lặp: " + name)
		}
		seen[name] = true
		switch name {
		case "--wait", "--counts", "--no-images":
			if hasValue {
				return o, cloneInvalid(name + " không nhận giá trị")
			}
			switch name {
			case "--wait":
				o.wait = true
			case "--counts":
				o.counts = true
			case "--no-images":
				o.noImages = true
			}
		case "--channel", "--priority", "--limit", "--timeout", "--api", "--cookie":
			if !hasValue {
				if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
					return o, cloneInvalid(name + " cần một giá trị")
				}
				i++
				value = args[i]
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return o, cloneInvalid(name + " không được để trống")
			}
			switch name {
			case "--channel":
				o.channel = value
			case "--priority":
				if value != string(domain.ClonePick) && value != string(domain.CloneBatch) {
					return o, cloneInvalid("--priority phải là pick|batch")
				}
				o.priority = domain.ClonePriority(value)
			case "--limit":
				n, err := strconv.Atoi(value)
				if err != nil || n < 1 || n > 100 {
					return o, cloneInvalid("--limit phải là số nguyên từ 1 tới 100")
				}
				o.limit = n
			case "--timeout":
				seconds, err := strconv.ParseFloat(value, 64)
				if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || seconds >= float64(1<<63-1)/float64(time.Second) {
					return o, cloneInvalid("--timeout phải là số giây dương")
				}
				o.timeout = time.Duration(seconds * float64(time.Second))
				if o.timeout <= 0 {
					return o, cloneInvalid("--timeout phải là số giây dương")
				}
			case "--api":
				o.api = value
			case "--cookie":
				o.cookie = value
			}
		default:
			return o, cloneInvalid("tham số không hợp lệ: " + name)
		}
	}

	allowed := map[string]bool{"--api": true, "--cookie": true}
	switch o.command {
	case "run", "batch":
		allowed["--channel"], allowed["--priority"], allowed["--no-images"] = true, true, true
		if o.command == "run" {
			allowed["--wait"], allowed["--timeout"] = true, true
		}
		if o.command == "run" && len(o.operands) != 1 || o.command == "batch" && len(o.operands) == 0 {
			return o, cloneInvalid("run cần đúng một URL hoặc jobId; batch cần ít nhất một URL")
		}
		if o.command == "batch" && o.channel == "" {
			return o, cloneInvalid("batch cần --channel <id> (hoặc --for <id>)")
		}
		if o.channel != "" {
			if o.wait || seen["--timeout"] {
				return o, cloneInvalid("video mới vào hàng chờ; dùng clone wait <jobId> khi mục đã có job")
			}
			for _, operand := range o.operands {
				u, err := url.Parse(operand)
				if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
					return o, cloneInvalid("nguồn video phải là URL http hoặc https hợp lệ")
				}
			}
		} else {
			if seen["--priority"] || o.noImages {
				return o, cloneInvalid("--priority/--no-images chỉ dùng khi xếp URL vào hàng chờ")
			}
			if err := validateCloneJobID(o.operands[0]); err != nil {
				return o, err
			}
		}
		if o.priority == "" {
			o.priority = domain.ClonePick
			if o.command == "batch" {
				o.priority = domain.CloneBatch
			}
		}
	case "status":
		allowed["--channel"], allowed["--limit"], allowed["--counts"] = true, true, true
		allowed["--wait"], allowed["--timeout"] = true, true
		if len(o.operands) > 1 {
			return o, cloneInvalid("status nhận tối đa một jobId")
		}
		if o.counts && (len(o.operands) != 0 || seen["--channel"] || seen["--limit"] || o.wait || seen["--timeout"]) {
			return o, cloneInvalid("--counts không kết hợp với jobId hoặc cờ lọc/chờ")
		}
		if len(o.operands) == 1 {
			if seen["--channel"] || seen["--limit"] {
				return o, cloneInvalid("--channel/--limit chỉ dùng cho danh sách status")
			}
			if err := validateCloneJobID(o.operands[0]); err != nil {
				return o, err
			}
		} else if o.wait || seen["--timeout"] {
			return o, cloneInvalid("chờ trạng thái cần một jobId")
		}
	default:
		if len(o.operands) != 1 {
			return o, cloneInvalid(o.command + " cần đúng một jobId")
		}
		if err := validateCloneJobID(o.operands[0]); err != nil {
			return o, err
		}
		if o.command == "wait" {
			allowed["--timeout"] = true
			o.wait = true
		}
	}
	for name := range seen {
		if !allowed[name] {
			return o, cloneInvalid(name + " không dùng với " + o.command)
		}
	}
	if seen["--timeout"] && !o.wait {
		return o, cloneInvalid("--timeout cần wait, --wait hoặc --watch")
	}
	if o.api == "" {
		o.api = strings.TrimSpace(getenv("PUBLIC_URL"))
		if o.api == "" {
			addr := strings.TrimSpace(getenv("VIDERSE_ADDR"))
			if addr == "" {
				addr = "127.0.0.1:8789"
			}
			o.api = "http://" + addr
		}
	}
	u, err := url.Parse(o.api)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return o, cloneInvalid("--api/PUBLIC_URL phải là URL http hoặc https, không có thông tin đăng nhập, query hay fragment")
	}
	o.api = strings.TrimRight(o.api, "/")
	if !seen["--cookie"] {
		o.cookie = strings.TrimSpace(getenv("VIDERSE_COOKIE"))
	}
	if strings.ContainsAny(o.cookie, "\r\n") {
		return o, cloneInvalid("cookie phiên không hợp lệ")
	}
	return o, nil
}

func validateCloneJobID(id string) error {
	if strings.TrimSpace(id) != id || id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\?#\r\n\t ") {
		return cloneInvalid("jobId không hợp lệ; URL mới cần --channel <id>")
	}
	return nil
}

type cloneHTTPClient struct {
	base         string
	cookie       string
	http         *http.Client
	pollInterval time.Duration
}

func (c *cloneHTTPClient) execute(ctx context.Context, o cloneOptions) (json.RawMessage, error) {
	if o.command == "batch" || o.command == "run" && o.channel != "" {
		return c.request(ctx, http.MethodPost, "/api/clone/enqueue", domain.CloneEnqueueRequest{
			ChannelID: o.channel, URLs: o.operands, Priority: o.priority, NoImages: o.noImages,
		})
	}
	if o.command == "status" && len(o.operands) == 0 {
		if o.counts {
			return c.request(ctx, http.MethodGet, "/api/clone/counts", nil)
		}
		q := url.Values{"limit": {strconv.Itoa(o.limit)}}
		if o.channel != "" {
			q.Set("channelId", o.channel)
		}
		return c.request(ctx, http.MethodGet, "/api/clone?"+q.Encode(), nil)
	}
	id := o.operands[0]
	path := "/api/clone/" + url.PathEscape(id)
	if o.command == "run" || o.command == "pause" || o.command == "resume" || o.command == "cancel" {
		result, err := c.request(ctx, http.MethodPost, path, struct {
			Action string `json:"action"`
		}{Action: o.command})
		if err != nil || !o.wait {
			return result, err
		}
	}
	if o.wait {
		return c.wait(ctx, id, o.timeout)
	}
	return c.request(ctx, http.MethodGet, path, nil)
}

// body accepts only the enqueue DTO or the action document built by execute.
func (c *cloneHTTPClient) request(ctx context.Context, method, path string, body interface{}) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var input io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("không mã hoá được yêu cầu: %w", err)
		}
		input = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, input)
	if err != nil {
		return nil, fmt.Errorf("không tạo được yêu cầu máy chủ: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("không gọi được máy chủ: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("không đọc được phản hồi máy chủ: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var envelope cloneCLIError
		if json.Unmarshal(raw, &envelope) == nil && envelope.Message != "" {
			return nil, &envelope
		}
		return nil, &cloneCLIError{Message: fmt.Sprintf("máy chủ trả HTTP %d", resp.StatusCode), Code: domain.CloneErrInternal}
	}
	if !json.Valid(raw) || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, &cloneCLIError{Message: "máy chủ trả kết quả JSON không hợp lệ", Code: domain.CloneErrInternal}
	}
	return json.RawMessage(raw), nil
}

func (c *cloneHTTPClient) wait(ctx context.Context, id string, timeout time.Duration) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		raw, err := c.request(ctx, http.MethodGet, "/api/clone/"+url.PathEscape(id), nil)
		if err != nil {
			return nil, err
		}
		var view domain.CloneJobView
		if err := json.Unmarshal(raw, &view); err != nil || view.ID != id {
			return nil, &cloneCLIError{Message: "máy chủ trả trạng thái job không hợp lệ", Code: domain.CloneErrInternal}
		}
		switch view.Status {
		case domain.CloneFailed:
			return nil, cloneFailedJob(view)
		case domain.CloneSucceeded, domain.CloneCancelled:
			return raw, nil
		case domain.CloneWaiting:
			// Quota, pause and automatic retries are not review checkpoints.
			if view.PausedAt == nil {
				for _, step := range view.Steps {
					if step.Status == domain.CloneWaiting && step.Error != nil && step.Error.Code == domain.CloneErrAwaitingReview {
						return raw, nil
					}
				}
			}
		case domain.ClonePending, domain.CloneRunning, domain.CloneStale:
		default:
			return nil, &cloneCLIError{Message: "máy chủ trả trạng thái job không hợp lệ", Code: domain.CloneErrInternal}
		}
		timer := time.NewTimer(c.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func cloneFailedJob(view domain.CloneJobView) error {
	e := &cloneCLIError{Message: "pipeline thất bại", Code: domain.CloneErrInternal}
	var stepName domain.CloneStep
	if view.ActiveStep != nil {
		stepName = *view.ActiveStep
	}
	for _, step := range view.Steps {
		if step.Status != domain.CloneFailed {
			continue
		}
		stepName = step.Step
		if step.Error != nil {
			e.Message, e.Code = step.Error.Message, step.Error.Code
			e.Recovery, e.RetryAt = step.Error.Recovery, step.Error.RetryAt
		}
		break
	}
	// These string-only fields cannot fail JSON encoding.
	e.Details, _ = json.Marshal(struct {
		JobID  string           `json:"jobId"`
		Step   domain.CloneStep `json:"step"`
		Status string           `json:"status"`
	}{JobID: view.ID, Step: stepName, Status: "failed"})
	return e
}

func cloneErrOut(stderr io.Writer, err error) int {
	envelope := &cloneCLIError{Message: err.Error(), Code: domain.CloneErrInternal}
	var cliErr *cloneCLIError
	if errors.As(err, &cliErr) {
		envelope = cliErr
	} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		envelope.Message = "đã dừng chờ job hoặc hết thời hạn"
		envelope.Code = domain.CloneErrInterrupted
	}
	if writeErr := json.NewEncoder(stderr).Encode(envelope); writeErr != nil {
		return 1
	}
	return 1
}
