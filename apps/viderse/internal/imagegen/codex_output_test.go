// Ca test chuyển từ src/server/clone/codex-image-output.test.ts của
// yt-factory: tách JSONL, gán session, capture reported-path, chống
// symlink/mơ hồ, base64 có trần, và đối chiếu owned-session.
package imagegen_test

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
)

type parserFixture struct {
	home   string
	parser *imagegen.CodexImageOutputParser
}

func newParserFixture(t *testing.T) parserFixture {
	t.Helper()
	home := t.TempDir()
	return parserFixture{home: home, parser: imagegen.NewCodexImageOutputParser(home)}
}

func mustPush(t *testing.T, p *imagegen.CodexImageOutputParser, chunk string) {
	t.Helper()
	if err := p.Push(chunk); err != nil {
		t.Fatalf("push: %v", err)
	}
}

func jsonLine(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// "split JSONL, non-JSON logs and two overlapping sessions stay attributed"
func TestSplitJSONLAndSessionAttribution(t *testing.T) {
	a := newParserFixture(t)
	b := newParserFixture(t)
	events := jsonLine(map[string]any{"type": "thread.started", "thread_id": "session-a"}) + "\n" +
		jsonLine(map[string]any{"type": "item.completed", "item": map[string]any{
			"type": "image_generation", "id": "call-a", "status": "completed",
			"result": base64.StdEncoding.EncodeToString(testPNG),
		}}) + "\n"
	mustPush(t, a.parser, "non-JSON log\n")
	for n := 0; n < len(events); n += 7 {
		end := n + 7
		if end > len(events) {
			end = len(events)
		}
		mustPush(t, a.parser, events[n:end])
	}
	mustPush(t, b.parser, `{"type":"thread.started","thread_id":"session-b"}`+"\n")
	if len(a.parser.Outputs) != 1 || a.parser.Outputs[0].SessionID != "session-a" {
		t.Fatalf("outputs a = %+v", a.parser.Outputs)
	}
	if len(b.parser.Outputs) != 0 {
		t.Fatalf("outputs b = %+v", b.parser.Outputs)
	}
	if err := a.parser.Push(`{"type":"thread.started","thread_id":"session-b"}` + "\n"); err == nil || !strings.Contains(err.Error(), "session_mismatch") {
		t.Fatalf("mong session_mismatch, được %v", err)
	}
}

// "exact reported path captures a saved image even after caller cancellation"
func TestReportedPathCapture(t *testing.T) {
	f := newParserFixture(t)
	png := filepath.Join(f.home, "generated_images", "session", "call.png")
	if err := os.MkdirAll(filepath.Dir(png), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(png, testPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	mustPush(t, f.parser, `{"type":"thread.started","thread_id":"session"}`+"\n")
	mustPush(t, f.parser, jsonLine(map[string]any{
		"type": "item.completed",
		"item": map[string]any{"type": "agent_message", "text": "Saved: " + png},
	}))
	if err := f.parser.Finish(); err != nil {
		t.Fatal(err)
	}
	if len(f.parser.Outputs) != 1 || f.parser.Outputs[0].Attribution != "reported-path" {
		t.Fatalf("outputs = %+v", f.parser.Outputs)
	}
	target := filepath.Join(f.home, "captured.png")
	if err := imagegen.CaptureImage(f.parser.Outputs[0], f.home, target); err != nil {
		t.Fatalf("capture: %v", err)
	}
	data, _ := os.ReadFile(target)
	if string(data) != string(testPNG) {
		t.Fatal("captured bytes khác PNG")
	}
}

// "symlink traversal, cross-session output and model mismatch fail closed"
func TestCaptureFailsClosed(t *testing.T) {
	f := newParserFixture(t)
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "call.png"), testPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(f.home, "generated_images"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(f.home, "generated_images", "session")); err != nil {
		t.Fatal(err)
	}
	err := imagegen.CaptureImage(imagegen.ImageOutput{
		SessionID: "session", CallID: "call", Attribution: "event",
		SavedPath: filepath.Join(f.home, "generated_images", "session", "call.png"),
	}, f.home, filepath.Join(f.home, "result.png"))
	if err == nil || !strings.Contains(err.Error(), "path_scope") {
		t.Fatalf("mong path_scope, được %v", err)
	}
	err = imagegen.CaptureImage(imagegen.ImageOutput{
		SessionID: "other", CallID: "call", Attribution: "event",
		SavedPath: filepath.Join(external, "call.png"),
	}, f.home, filepath.Join(f.home, "result.png"))
	if err == nil || !strings.Contains(err.Error(), "path_scope") {
		t.Fatalf("mong path_scope, được %v", err)
	}
	err = imagegen.CaptureImage(imagegen.ImageOutput{
		SessionID: "session", CallID: "call", Attribution: "event",
		Base64: base64.StdEncoding.EncodeToString(testPNG), Model: "other-model",
	}, f.home, filepath.Join(f.home, "result.png"))
	if err == nil || !strings.Contains(err.Error(), "model_mismatch") {
		t.Fatalf("mong model_mismatch, được %v", err)
	}
}

// "bounded base64 rejects corrupt and non-image payloads"
func TestDecodeBase64RejectsCorrupt(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(testPNG)
	got, err := imagegen.DecodeImageBase64(b64)
	if err != nil || string(got) != string(testPNG) {
		t.Fatalf("decode: %v", err)
	}
	truncated := testPNG
	if len(truncated) > 40 {
		truncated = truncated[:40]
	}
	for _, value := range []string{
		"!!!!",
		"dGVzdA==",
		base64.StdEncoding.EncodeToString(truncated),
		strings.Repeat("A", 46*1024*1024),
	} {
		if _, err := imagegen.DecodeImageBase64(value); err == nil || !strings.Contains(err.Error(), "invalid_image") {
			t.Fatalf("value %q: mong invalid_image, được %v", value[:20], err)
		}
	}
}

// "rollout fallback only reads this session inside the isolated home"
func TestRolloutScope(t *testing.T) {
	f := newParserFixture(t)
	mustPush(t, f.parser, `{"type":"thread.started","thread_id":"session"}`+"\n")
	sessions := filepath.Join(f.home, "sessions")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(sessions, "rollout-session.jsonl")
	content := jsonLine(map[string]any{
		"type": "response_item",
		"payload": map[string]any{
			"type": "image_generation_call", "id": "call", "status": "completed",
			"result": base64.StdEncoding.EncodeToString(testPNG),
		},
	}) + "\n"
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := imagegen.ReadImageRollout(file, f.parser); err != nil {
		t.Fatalf("rollout: %v", err)
	}
	if len(f.parser.Outputs) != 1 || f.parser.Outputs[0].Attribution != "rollout" {
		t.Fatalf("outputs = %+v", f.parser.Outputs)
	}
	wrong := filepath.Join(sessions, "rollout-other.jsonl")
	if err := os.WriteFile(wrong, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := imagegen.ReadImageRollout(wrong, f.parser); err == nil || !strings.Contains(err.Error(), "rollout_scope") {
		t.Fatalf("mong rollout_scope, được %v", err)
	}
}

// "unrelated tools and quota reset are machine-readable, not prose refusal"
func TestForbiddenToolsAndQuotaReset(t *testing.T) {
	f := newParserFixture(t)
	mustPush(t, f.parser, `{"type":"thread.started","thread_id":"session"}`+"\n")
	for _, typ := range []string{"command_execution", "mcp_tool_call", "web_search", "collab_tool_call"} {
		mustPush(t, f.parser, jsonLine(map[string]any{
			"type": "item.started", "item": map[string]any{"type": typ},
		})+"\n")
	}
	mustPush(t, f.parser, jsonLine(map[string]any{
		"type": "item.completed",
		"item": map[string]any{
			"type": "image_generation",
			"failure": map[string]any{
				"type":      "usage_limit_exceeded",
				"resets_at": 2000000000,
			},
		},
	})+"\n")
	if len(f.parser.ForbiddenTools) != 4 {
		t.Fatalf("forbidden = %v", f.parser.ForbiddenTools)
	}
	if f.parser.RetryAt == nil || *f.parser.RetryAt != 2000000000000 {
		t.Fatalf("retryAt = %v", f.parser.RetryAt)
	}
	if len(f.parser.Outputs) != 0 {
		t.Fatalf("outputs = %+v", f.parser.Outputs)
	}
}

// "saved-before-cancellation is recovered only from the unique owned
// session, never newest"
func TestOwnedSessionRecovery(t *testing.T) {
	f := newParserFixture(t)
	dir := filepath.Join(f.home, "generated_images", "owned-session")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "call_owned.png"), testPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := imagegen.FindOwnedSessionImage(f.home, "owned-session")
	if err != nil || out == nil {
		t.Fatalf("find: %v", err)
	}
	if out.CallID != "call_owned" || out.Attribution != "owned-session-file" {
		t.Fatalf("out = %+v", out)
	}
	if other, err := imagegen.FindOwnedSessionImage(f.home, "other-session"); err != nil || other != nil {
		t.Fatalf("other session = %+v, %v", other, err)
	}
	if err := imagegen.CaptureImage(*out, f.home, filepath.Join(f.home, "captured.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "call_newer.png"), testPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := imagegen.FindOwnedSessionImage(f.home, "owned-session"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("mong ambiguous, được %v", err)
	}
}

// "PNG decompression-bomb dimensions and symlinked session directories are
// rejected"
func TestBombDimensionsAndSymlinkedSessions(t *testing.T) {
	huge := append([]byte{}, testPNG...)
	binary.BigEndian.PutUint32(huge[16:20], 100000)
	if _, err := imagegen.DecodeImageBase64(base64.StdEncoding.EncodeToString(huge)); err == nil || !strings.Contains(err.Error(), "invalid_image") {
		t.Fatalf("mong invalid_image, được %v", err)
	}
	f := newParserFixture(t)
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(f.home, "generated_images"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(f.home, "generated_images", "owned-session")); err != nil {
		t.Fatal(err)
	}
	if _, err := imagegen.FindOwnedSessionImage(f.home, "owned-session"); err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("mong scope, được %v", err)
	}
}

// ---- kiểm PNG & capture bổ sung ----

func TestValidatePngRejectsGarbage(t *testing.T) {
	f := newParserFixture(t)
	bad := filepath.Join(f.home, "bad.png")
	if err := os.WriteFile(bad, []byte("not a png at all, definitely not"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := imagegen.ValidatePngFile(bad); err == nil {
		t.Fatal("mong lỗi PNG")
	}
	good := filepath.Join(f.home, "good.png")
	if err := os.WriteFile(good, testPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := imagegen.ValidatePngFile(good); err != nil {
		t.Fatalf("png hợp lệ bị từ chối: %v", err)
	}
}

// Capture không bao giờ ghi đè target có sẵn (flag "wx" của bản cũ).
func TestCaptureNeverOverwrites(t *testing.T) {
	f := newParserFixture(t)
	target := filepath.Join(f.home, "exists.png")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := imagegen.CaptureImage(imagegen.ImageOutput{
		SessionID: "s", CallID: "c", Attribution: "event",
		Base64: base64.StdEncoding.EncodeToString(testPNG),
	}, f.home, target)
	if err == nil {
		t.Fatal("mong lỗi tồn tại")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "keep" {
		t.Fatal("target bị ghi đè")
	}
}
