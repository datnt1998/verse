package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/datnt1998/verse/apps/noverse/internal/llm/wire"
)

// toolStream normalizes Responses function calls by stable item identity.
// Names, call IDs and arguments can arrive in different events or in two
// half-items sharing a call ID. A completed call is emitted exactly once,
// after its full item arrives or the provider finishes the response.
// Transport errors never promote incomplete arguments to executable calls.
type toolStream struct {
	inner   litellm.Stream
	pending map[string]*toolCallBuf
	order   []string
	aliases map[string]string
	// emitted ghi các call_id đã phát để một lời gọi không bao giờ xuất hiện
	// hai lần trong cùng phản hồi (backend có thể gửi thêm added/item.done
	// muộn sau khi lời gọi đã được xả).
	emitted map[string]bool
	out     []litellm.Event
	done    bool
	seq     int
}

type toolCallBuf struct {
	id          string
	itemID      string
	name        string
	outputIndex *int
	args        []byte
	flushed     bool
}

func wrapToolStream(inner litellm.Stream) litellm.Stream {
	return &toolStream{
		inner:   inner,
		pending: make(map[string]*toolCallBuf),
		aliases: make(map[string]string),
		emitted: make(map[string]bool),
	}
}

func (s *toolStream) Close() error { return s.inner.Close() }

func (s *toolStream) Next() (litellm.Event, error) {
	for {
		if len(s.out) > 0 {
			ev := s.out[0]
			s.out = s.out[1:]
			return ev, nil
		}
		if s.done {
			return nil, io.EOF
		}
		ev, err := s.inner.Next()
		if err != nil {
			// Lỗi giữa stream: arguments có thể đã cụt, KHÔNG được phát thành lời
			// gọi hoàn chỉnh (agentcore sẽ bỏ lượt này vì lỗi stream).
			if err != io.EOF {
				s.done = true
				return nil, err
			}
			// EOF: backend không gửi nốt output_item.done — xả phần đã ghép đủ.
			s.flushAll()
			s.done = true
			continue
		}
		s.handle(ev)
	}
}

func (s *toolStream) handle(ev litellm.Event) {
	switch e := ev.(type) {
	case litellm.ToolUseStart:
		buf := s.get(e.ID, e.ItemID, e.OutputIndex)
		if buf == nil {
			return
		}
		buf.adoptID(e.ID)
		if buf.itemID == "" {
			buf.itemID = e.ItemID
		}
		if buf.outputIndex == nil {
			buf.outputIndex = e.OutputIndex
		}
		if buf.name == "" {
			buf.name = e.Name
		}
	case litellm.ToolUseDelta:
		buf := s.get(e.ID, e.ItemID, e.OutputIndex)
		if buf == nil {
			return
		}
		buf.adoptID(e.ID)
		if buf.itemID == "" {
			buf.itemID = e.ItemID
		}
		if buf.outputIndex == nil {
			buf.outputIndex = e.OutputIndex
		}
		buf.args = append(buf.args, e.ArgumentsDelta...)
	case litellm.ToolUseDone:
		buf := s.get(e.ID, e.ItemID, e.OutputIndex)
		if buf == nil {
			return
		}
		buf.adoptID(e.ID)
		if buf.itemID == "" {
			buf.itemID = e.ItemID
		}
		if buf.outputIndex == nil {
			buf.outputIndex = e.OutputIndex
		}
	case litellm.ProviderEvent:
		if e.Name == "response.output_item.done" {
			s.handleItemDone(e.Raw)
			return
		}
		s.out = append(s.out, ev)
	case litellm.DoneEvent, litellm.ErrorEvent:
		// Không còn output_item.done nào nữa: xả mọi item còn treo trước khi
		// chuyển sự kiện kết thúc xuống dưới.
		s.flushAll()
		s.out = append(s.out, ev)
	default:
		s.out = append(s.out, ev)
	}
}

// get tìm buffer theo bất kỳ khoá nào (call_id, item_id, output_index); sự kiện
// của cùng một function_call có thể đổi khoá khi call_id chỉ đến muộn.
func (s *toolStream) get(id, itemID string, outputIndex *int) *toolCallBuf {
	for _, key := range toolKeys(id, itemID, outputIndex) {
		if canonical := s.aliases[key]; canonical != "" {
			if buf := s.pending[canonical]; buf != nil {
				return buf
			}
		}
		if buf := s.pending[key]; buf != nil {
			return buf
		}
	}
	keys := toolKeys(id, itemID, outputIndex)
	if len(keys) == 0 {
		return nil
	}
	buf := &toolCallBuf{id: id, itemID: itemID}
	s.pending[keys[0]] = buf
	s.order = append(s.order, keys[0])
	for _, key := range keys {
		s.aliases[key] = keys[0]
	}
	return buf
}

// adoptID chọn call_id làm ID canonical: delta có thể đến trước
// output_item.added nên lúc đó chỉ biết item id ("fc_…"), còn call_id thật
// ("call_…") chỉ xuất hiện sau. ID nào khác item id thì thắng.
func (b *toolCallBuf) adoptID(id string) {
	if id == "" {
		return
	}
	if b.id == "" || (id != b.itemID && b.id == b.itemID) {
		b.id = id
	}
}

func toolKeys(id, itemID string, outputIndex *int) []string {
	var keys []string
	if itemID != "" {
		keys = append(keys, "item:"+itemID)
	}
	if id != "" {
		keys = append(keys, "id:"+id)
	}
	// output_index là khoá ổn định cuối cùng: backend có thể gửi delta chỉ kèm
	// output_index (không item_id), hoặc added với item id khác item_id của
	// delta. Thiếu khoá này thì name và arguments rơi vào hai buffer.
	if outputIndex != nil {
		keys = append(keys, "out:"+strconv.Itoa(*outputIndex))
	}
	return keys
}

// callID trả id canonical đang biết của buffer (call_id, không thì item id).
func (b *toolCallBuf) callID() string {
	if b.id != "" {
		return b.id
	}
	return b.itemID
}

// handleItemDone nhận item hoàn chỉnh trong response.output_item.done — đây là
// nguồn chính xác của function_call trên backend Codex (name/call_id có thể
// chỉ xuất hiện ở đây). Item không phải function_call được chuyển nguyên văn.
func (s *toolStream) handleItemDone(raw json.RawMessage) {
	var ev struct {
		Item struct {
			ID        string `json:"id"`
			Type      string `json:"type"`
			Name      string `json:"name"`
			CallID    string `json:"call_id"`
			Arguments string `json:"arguments"`
		} `json:"item"`
		OutputIndex *int `json:"output_index"`
	}
	if json.Unmarshal(raw, &ev) != nil || ev.Item.Type != "function_call" {
		s.out = append(s.out, litellm.ProviderEvent{Name: "response.output_item.done", Raw: raw})
		return
	}
	// Ưu tiên khớp theo cả hai khoá: delta có thể đã tạo buffer chỉ với call_id
	// (khi đó item_id chưa biết), còn added thì tạo với item_id.
	buf := s.get(ev.Item.CallID, ev.Item.ID, ev.OutputIndex)
	if buf == nil {
		// Không đủ id để agentcore ghép lời gọi — giữ nguyên sự kiện gốc thay vì
		// nuốt mất lời gọi.
		s.out = append(s.out, litellm.ProviderEvent{Name: "response.output_item.done", Raw: raw})
		return
	}
	if buf.flushed {
		return
	}
	if ev.Item.CallID != "" {
		buf.id = ev.Item.CallID
	}
	if buf.id == "" {
		buf.id = ev.Item.ID
	}
	if buf.itemID == "" {
		buf.itemID = ev.Item.ID
	}
	if ev.OutputIndex != nil {
		buf.outputIndex = ev.OutputIndex
	}
	name := ev.Item.Name
	if name == "" {
		name = buf.name
	}
	args := ev.Item.Arguments
	if args == "" {
		args = string(buf.args)
	}
	s.emitCall(buf, name, args)
}

// emitCall phát bộ ba canonical cho một function_call, gộp mọi buffer cùng
// lời gọi trước khi phát.
//
// Lời gọi thiếu name bị BỎ (không phát start/delta/done): agentcore sẽ chạy
// "tool \"\" not found" rồi ghi lời gọi rỗng tên vào lịch sử, và lượt sau
// litellm từ chối cả request ("tool use … missing name" / "duplicate tool use
// id") — đúng vòng lặp đã làm hỏng phiên writer (tui.log 2026-10-26).
func (s *toolStream) emitCall(buf *toolCallBuf, name, args string) {
	if buf.flushed {
		return
	}
	buf.flushed = true
	if name == "" {
		name = buf.name
	}
	if args == "" {
		args = string(buf.args)
	}
	id := buf.callID()
	// Gộp nửa name và nửa arguments của CÙNG một lời gọi (backend tách item id
	// của added khỏi item_id của delta, hoặc dùng item id lại): một call chỉ
	// được phát một lần.
	group := []*toolCallBuf{buf}
	if id != "" {
		for _, other := range s.pending {
			if other == buf || other.flushed || other.callID() != id {
				continue
			}
			other.flushed = true
			group = append(group, other)
			if name == "" {
				name = other.name
			}
			args = mergeToolArgs(args, string(other.args))
			if buf.itemID == "" {
				buf.itemID = other.itemID
			}
			if buf.outputIndex == nil {
				buf.outputIndex = other.outputIndex
			}
		}
	}
	for _, b := range group {
		s.retire(b)
	}
	if name == "" {
		return
	}
	args = collapseDuplicateArgs(args)
	if id == "" {
		// Không có id nào: vẫn phải có một id hợp lệ để lịch sử phát lại không
		// vĩnh viễn bị litellm từ chối; seq bảo đảm không trùng giữa các call.
		s.seq++
		id = fmt.Sprintf("call_seq_%d", s.seq)
	}
	// Mỗi call_id chỉ được phát một lần: hai tool_use cùng id làm litellm từ
	// chối cả request ("duplicate tool use id") — chính là dạng lịch sử đã lưu
	// lại trong phiên writer chương 2.
	if s.emitted[id] {
		return
	}
	s.emitted[id] = true
	buf.id = id
	s.out = append(s.out,
		litellm.ToolUseStart{ID: id, Name: name, OutputIndex: buf.outputIndex, ItemID: buf.itemID},
		litellm.ToolUseDelta{ID: id, OutputIndex: buf.outputIndex, ItemID: buf.itemID, ArgumentsDelta: []byte(args)},
		litellm.ToolUseDone{ID: id, OutputIndex: buf.outputIndex, ItemID: buf.itemID},
	)
}

// retire gỡ buffer khỏi mọi khoá đã đăng ký. Bắt buộc phải gỡ: backend có thể
// dùng lại item id cho lời gọi kế tiếp, và buffer đã phát không được hút sự
// kiện của lời gọi mới (nếu không, lời gọi thứ hai biến mất khỏi lượt).
func (s *toolStream) retire(buf *toolCallBuf) {
	for key, pending := range s.pending {
		if pending == buf {
			delete(s.pending, key)
		}
	}
	for key, canonical := range s.aliases {
		if s.pending[canonical] == nil {
			delete(s.aliases, key)
		}
	}
}

// mergeToolArgs ghép arguments của hai nửa cùng một lời gọi. Chỉ một nửa có dữ
// liệu là trường hợp thường gặp; nếu cả hai đều có thì ưu tiên bản JSON hợp lệ
// (bản dài hơn khi cùng hợp lệ) và không bao giờ nối chuỗi thô — nối sẽ tạo
// JSON hỏng.
func mergeToolArgs(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	case a == b:
		return a
	}
	validA, validB := json.Valid([]byte(a)), json.Valid([]byte(b))
	switch {
	case validA && !validB:
		return a
	case validB && !validA:
		return b
	case len(b) > len(a):
		return b
	}
	return a
}

// collapseDuplicateArgs gỡ arguments bị gửi hai lần cho cùng một lời gọi.
// Khi backend chỉ gửi output_index (không item_id), litellm không đánh dấu
// toolSeen nên response.function_call_arguments.done phát lại TOÀN BỘ
// arguments sau các delta đã phát — buffer nhận cả hai. JSON hợp lệ không bao
// giờ là bản lặp của chính nó, nên chỉ cần gỡ khi bản lặp không hợp lệ.
func collapseDuplicateArgs(args string) string {
	if len(args) < 2 || len(args)%2 != 0 || json.Valid([]byte(args)) {
		return args
	}
	half := len(args) / 2
	if args[:half] == args[half:] {
		return args[:half]
	}
	return args
}

// flushAll xả mọi function_call chưa có output_item.done (backend tắt sự kiện
// đó hoặc stream gãy): ghép name/args tốt nhất đã có, không bỏ sót lời gọi.
func (s *toolStream) flushAll() {
	seen := make(map[*toolCallBuf]bool, len(s.pending))
	for _, key := range s.order {
		buf := s.pending[key]
		if buf == nil || buf.flushed || seen[buf] {
			continue
		}
		seen[buf] = true
		s.emitCall(buf, buf.name, string(buf.args))
	}
	s.order = nil
}
