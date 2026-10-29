package anthropicoauth

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// unprefixResponse bỏ tiền tố tên tool (applyToolPrefix) khỏi phản hồi để phần còn lại của
// ainovel chỉ thấy tên tool gốc: JSON thường (content[].tool_use) và SSE (content_block_start).
func unprefixResponse(resp *http.Response) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || resp.Body == nil {
		return
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	switch {
	case strings.Contains(ct, "text/event-stream"):
		resp.Body = &sseUnprefixer{src: bufio.NewReader(resp.Body), closer: resp.Body}
	case strings.Contains(ct, "json"):
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(raw), errReader{err}))
			return
		}
		raw = unprefixMessage(raw)
		resp.Body = io.NopCloser(bytes.NewReader(raw))
		resp.ContentLength = int64(len(raw))
		resp.Header.Set("Content-Length", strconv.Itoa(len(raw)))
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

// unprefixMessage sửa name của các block tool_use trong một Message JSON; không phải tool_use
// hay không parse được thì trả nguyên bản.
func unprefixMessage(raw []byte) []byte {
	if !bytes.Contains(raw, []byte(`"tool_use"`)) {
		return raw
	}
	var msg map[string]json.RawMessage
	if json.Unmarshal(raw, &msg) != nil {
		return raw
	}
	var blocks []map[string]json.RawMessage
	if json.Unmarshal(msg["content"], &blocks) != nil {
		return raw
	}
	changed := false
	for _, b := range blocks {
		if jsonString(b["type"]) == "tool_use" {
			b["name"] = encodeJSON(stripToolPrefix(jsonString(b["name"])))
			changed = true
		}
	}
	if !changed {
		return raw
	}
	msg["content"] = encodeJSON(blocks)
	return encodeJSON(msg)
}

// sseUnprefixer đọc SSE theo dòng và sửa tên tool ở sự kiện content_block_start (nơi duy nhất
// Anthropic gửi tên tool trong stream); mọi dòng khác đi qua nguyên vẹn.
type sseUnprefixer struct {
	src     *bufio.Reader
	closer  io.Closer
	pending []byte
	err     error
}

func (s *sseUnprefixer) Read(p []byte) (int, error) {
	for len(s.pending) == 0 {
		if s.err != nil {
			return 0, s.err
		}
		line, err := s.src.ReadBytes('\n')
		s.pending, s.err = unprefixSSELine(line), err
	}
	n := copy(p, s.pending)
	s.pending = s.pending[n:]
	return n, nil
}

func (s *sseUnprefixer) Close() error { return s.closer.Close() }

func unprefixSSELine(line []byte) []byte {
	if !bytes.HasPrefix(line, []byte("data:")) || !bytes.Contains(line, []byte(`"content_block_start"`)) || !bytes.Contains(line, []byte(`"tool_use"`)) {
		return line
	}
	payload := bytes.TrimSpace(line[len("data:"):])
	var ev map[string]json.RawMessage
	if json.Unmarshal(payload, &ev) != nil || jsonString(ev["type"]) != "content_block_start" {
		return line
	}
	var block map[string]json.RawMessage
	if json.Unmarshal(ev["content_block"], &block) != nil || jsonString(block["type"]) != "tool_use" {
		return line
	}
	block["name"] = encodeJSON(stripToolPrefix(jsonString(block["name"])))
	ev["content_block"] = encodeJSON(block)
	out := append([]byte("data: "), encodeJSON(ev)...)
	if bytes.HasSuffix(line, []byte("\r\n")) {
		return append(out, '\r', '\n')
	}
	if bytes.HasSuffix(line, []byte("\n")) {
		return append(out, '\n')
	}
	return out
}
