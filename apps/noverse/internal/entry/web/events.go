package web

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/noverse/internal/webauth"
)

// ── 全局事件流 GET /api/events（16-feedback-refresh §2/§4）──
//
// 网关唯一的 SSE：job.*（jobs.go）与各类失效事件（library/project/story/config/
// account/session）共用。每条事件带递增 id，环形缓冲保留最近 eventRing 条供
// Last-Event-ID 续传；缓冲已滚过续传点时先发 reset，客户端据此全部失效重取。

// eventRing 是续传缓冲容量。
const eventRing = 500

// eventPing 是心跳间隔。iOS Safari 与家用代理会掐断长时间没有字节的
// HTTP 流，15 s 一次注释心跳在这两类链路上都能活下来。
const eventPing = 15 * time.Second

// eventWriteTimeout 是单帧写超时：手机 Wi-Fi 掉线时 TCP 半开连接不会报错，
// 没有写截止时间就会永远卡在 Write 上，把订阅者留在 map 里。
const eventWriteTimeout = 15 * time.Second

// eventPingComment 是 SSE 注释行，客户端与代理都忽略，但会刷新空闲计时。
const eventPingComment = ": ping\n\n"

// deadlineWriter 给 SSE 响应装写截止时间。测试里的假 writer 不支持
// ResponseController，SetWriteDeadline 返回错误时静默忽略。
type deadlineWriter struct {
	w  io.Writer
	rc *http.ResponseController
}

func newDeadlineWriter(w http.ResponseWriter) deadlineWriter {
	return deadlineWriter{w: w, rc: http.NewResponseController(w)}
}

func (d deadlineWriter) Write(p []byte) (int, error) {
	_ = d.rc.SetWriteDeadline(time.Now().Add(eventWriteTimeout))
	n, err := d.w.Write(p)
	if err != nil {
		return n, err
	}
	_ = d.rc.SetWriteDeadline(time.Time{})
	return n, nil
}

// liveFrame 是 worker/audio 实况帧在全局流里的信封；kind 与 worker SSE 的
// event 名一致（snapshot · event · delta · clear · cocreate · job · done…）。
// 走瞬时广播：不进 ring、不带 id，重连后靠快照 + 正文尾巴重建。
type liveFrame struct {
	Project string          `json:"project"`
	Story   string          `json:"story"`
	Kind    string          `json:"kind"`
	Data    json.RawMessage `json:"data"`
}

// liveDataType 标记实况帧类型（/api/events 上唯一的 event 名）。
const liveDataType = "live"

func liveJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

// Event 是一条全局事件。可见性：User 非空 → 只发给该用户名；否则 Story 非空 →
// 需可读 Project/Story；否则 Project 非空 → 需可读项目；都空 → 所有登录用户。
type Event struct {
	ID        uint64
	Type      string
	Project   string
	Story     string
	User      string
	AdminsToo bool
	Data      any
}

func (e Event) visibleTo(u webauth.User) bool {
	switch {
	case u.Disabled:
		return false
	case e.User != "":
		return u.Username == e.User || (e.AdminsToo && u.Role == webauth.RoleAdmin)
	case e.Story != "":
		return u.CanRead(e.Project + "/" + e.Story)
	case e.Project != "":
		return u.CanReadProject(e.Project)
	default:
		return true
	}
}

// EventSub 是一个 /api/events 连接。Watch 为客户端声明正在看的范围
// （"p/s" 或 "p"，?watch= 与 SetWatch 更新），供文件监视只扫有人看的故事。
type EventSub struct {
	ID    string
	user  webauth.User
	watch map[string]bool
	ch    chan Event
}

// EventHub 是全局事件中心。
type EventHub struct {
	mu        sync.Mutex
	seq       uint64
	ring      []Event
	ringStart int // index of the oldest retained event
	subs      map[string]*EventSub
	onWatch   []func()
	onEvent   []func(Event)
	streams   map[string]string            // "p/s" → 流式正文尾巴（瞬时，不进 ring）
	events    map[string][]json.RawMessage // "p/s" → 最近实况 event 帧（补发用）
}

func newEventHub() *EventHub {
	return &EventHub{subs: map[string]*EventSub{}, ring: make([]Event, 0, eventRing)}
}

// events 惰性取全局事件中心（测试里有直接 &Gateway{} 的构造）。
func (g *Gateway) events() *EventHub {
	g.eventsOnce.Do(func() {
		if g.eventHub == nil {
			g.eventHub = newEventHub()
		}
	})
	return g.eventHub
}

// Publish 分配 id、写入续传缓冲并扇出给可见的订阅者；返回事件 id。
// 慢订阅者（缓冲满）被断开，客户端会带 Last-Event-ID 重连续传。
func (h *EventHub) Publish(ev Event) uint64 {
	h.mu.Lock()
	h.seq++
	ev.ID = h.seq
	if len(h.ring) < eventRing {
		h.ring = append(h.ring, ev)
	} else {
		h.ring[h.ringStart] = ev
		h.ringStart = (h.ringStart + 1) % eventRing
	}
	for id, sub := range h.subs {
		if !ev.visibleTo(sub.user) {
			continue
		}
		select {
		case sub.ch <- ev:
		default:
			delete(h.subs, id)
			close(sub.ch)
		}
	}
	hooks := h.onEvent
	h.mu.Unlock()
	for _, fn := range hooks {
		fn(ev)
	}
	return ev.ID
}

// Broadcast 扇出一条不进续传缓冲、不分配 id 的瞬时事件：用于 worker/audio
// 的高频实况帧（delta 每秒几十帧），它们不该把 500 条的续传环挤掉。
func (h *EventHub) Broadcast(ev Event) {
	h.mu.Lock()
	for id, sub := range h.subs {
		if !ev.visibleTo(sub.user) {
			continue
		}
		select {
		case sub.ch <- ev:
		default:
			delete(h.subs, id)
			close(sub.ch)
		}
	}
	h.mu.Unlock()
}

// Backlog 返回关注故事的实况补发帧（最近事件 · clear · 正文尾巴），与旧
// worker 直连流的 reset→replay→clear→delta 等价：只补不擦，客户端按 id 合并。
func (h *EventHub) Backlog(keys []string) []Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.backlogLocked(keys)
}

func (h *EventHub) backlogLocked(keys []string) []Event {
	var out []Event
	for _, key := range keys {
		project, story, ok := strings.Cut(key, "/")
		if !ok {
			continue
		}
		frame := func(kind string, data json.RawMessage) {
			out = append(out, Event{Type: liveDataType, Project: project, Story: story,
				Data: liveFrame{Project: project, Story: story, Kind: kind, Data: data}})
		}
		if list := h.events[key]; len(list) > 0 {
			frame("events", liveJSON(list))
		}
		if text, seen := h.streams[key]; seen {
			frame("clear", liveJSON(map[string]any{}))
			if text != "" {
				frame("delta", liveJSON(map[string]string{"text": text}))
			}
		}
	}
	return out
}

// SetLiveStream 记录某故事当前的流式正文尾巴，供新连接/重连时重建；
// 空串表示已 clear。仅 worker 泵写入，续传时由 Backlog 读出。
func (h *EventHub) SetLiveStream(key, text string) {
	h.mu.Lock()
	if h.streams == nil {
		h.streams = map[string]string{}
	}
	h.streams[key] = text
	h.mu.Unlock()
}

// liveEventCap 是每故事为“新连接补发”保留的实况事件条数（与旧 worker 直连流的
// ring 语义一致，只是不进全局续传环）。
const liveEventCap = 200

// AppendLiveEvent 记录一条实况 event 帧，供新标签页/重连补发。
func (h *EventHub) AppendLiveEvent(key string, frame json.RawMessage) {
	h.mu.Lock()
	if h.events == nil {
		h.events = map[string][]json.RawMessage{}
	}
	list := append(h.events[key], frame)
	if len(list) > liveEventCap {
		list = list[len(list)-liveEventCap:]
	}
	h.events[key] = list
	h.mu.Unlock()
}

// ResetLiveEvents 清空某故事的实况事件（worker 发 reset，说明它换了一轮）。
func (h *EventHub) ResetLiveEvents(key string) {
	h.mu.Lock()
	delete(h.events, key)
	h.mu.Unlock()
}

// ── 实况帧（worker → 全局 /api/events）──
//
// 手机（尤其 iOS）每个源只能开 6 条 HTTP/1.1 连接，而旧实现给每个故事再开一条
// worker SSE：一个标签页同时挂 /api/events + /events/live + /audio/events，
// 加上请求就排不上队 —— 页面表现成“数据一会儿没了又回来”。现在 worker 帧由
// 网关自己的泵读取（本来就是任务聚合用的那条流），再以瞬时事件广播到全局流，
// 每个标签页只留一条 SSE。

// liveCoalesce 把逐 token 的 delta 合并成 ≥80 ms 一帧，防止高频帧挤爆慢订阅者。
const liveCoalesce = 80 * time.Millisecond

const liveStreamCap = 64 * 1024

type liveMux struct {
	g         *Gateway
	key       string
	project   string
	story     string
	delta     strings.Builder
	cocreate  strings.Builder
	lastFlush time.Time
	stream    strings.Builder
}

func newLiveMux(g *Gateway, project, story string) *liveMux {
	return &liveMux{g: g, key: project + "/" + story, project: project, story: story}
}

// frame 处理一条 worker SSE 帧：delta/cocreate 合帧后再广播，其余原样广播。
func (m *liveMux) frame(kind string, data []byte) {
	switch kind {
	case "delta":
		text := frameText(data)
		m.delta.WriteString(text)
		m.appendStream(text)
		m.flush(false)
	case "cocreate":
		m.cocreate.WriteString(frameText(data))
		m.flush(false)
	default:
		m.flush(true)
		switch kind {
		case "clear":
			m.stream.Reset()
			m.g.events().SetLiveStream(m.key, "")
		case "reset":
			m.stream.Reset()
			m.g.events().SetLiveStream(m.key, "")
			m.g.events().ResetLiveEvents(m.key)
		case "event":
			m.g.events().AppendLiveEvent(m.key, json.RawMessage(data))
		}
		m.send(kind, data)
	}
}

func (m *liveMux) flush(force bool) {
	if !force && time.Since(m.lastFlush) < liveCoalesce {
		return
	}
	m.lastFlush = time.Now()
	if m.delta.Len() > 0 {
		text := m.delta.String()
		m.delta.Reset()
		m.send("delta", liveJSON(map[string]string{"text": text}))
	}
	if m.cocreate.Len() > 0 {
		text := m.cocreate.String()
		m.cocreate.Reset()
		m.send("cocreate", liveJSON(map[string]string{"text": text}))
	}
}

func (m *liveMux) appendStream(text string) {
	m.stream.WriteString(text)
	if m.stream.Len() > liveStreamCap {
		trimmed := m.stream.String()[m.stream.Len()-liveStreamCap:]
		m.stream.Reset()
		m.stream.WriteString(trimmed)
	}
	m.g.events().SetLiveStream(m.key, m.stream.String())
}

func (m *liveMux) send(kind string, data json.RawMessage) {
	m.g.events().Broadcast(Event{Type: liveDataType, Project: m.project, Story: m.story, Data: liveFrame{Project: m.project, Story: m.story, Kind: kind, Data: data}})
}

// frameText 提取 delta/cocreate 帧里的正文；坏帧当空串。
func frameText(data []byte) string {
	var v struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(data, &v) != nil {
		return ""
	}
	return v.Text
}

func (h *EventHub) subscribe(user webauth.User, lastID uint64, watch []string) (sub *EventSub, replay []Event, stale bool) {
	sub = &EventSub{ID: rand.Text(), user: user, watch: watchSet(watch), ch: make(chan Event, 256)}
	h.mu.Lock()
	if lastID > 0 {
		if lastID > h.seq || (len(h.ring) > 0 && lastID < h.ring[h.ringStart].ID-1) || (len(h.ring) == 0 && lastID < h.seq) {
			stale = true
		} else {
			for i := range len(h.ring) {
				ev := h.ring[(h.ringStart+i)%len(h.ring)]
				if ev.ID > lastID && ev.visibleTo(user) {
					replay = append(replay, ev)
				}
			}
		}
	}
	h.subs[sub.ID] = sub
	hooks := h.onWatch
	h.mu.Unlock()
	if len(sub.watch) > 0 {
		runHooks(hooks)
	}
	return sub, replay, stale
}

func (h *EventHub) unsubscribe(sub *EventSub) {
	h.mu.Lock()
	_, ok := h.subs[sub.ID]
	if ok {
		delete(h.subs, sub.ID)
		close(sub.ch)
	}
	hooks := h.onWatch
	h.mu.Unlock()
	if ok && len(sub.watch) > 0 {
		runHooks(hooks)
	}
}

// SetWatch 替换订阅者的关注范围；订阅者不存在或不属于该用户返回 false。
// 新加的故事立刻补发实况尾部（最近事件 + 正文），否则要等到下一次重连才看到。
func (h *EventHub) SetWatch(subID, username string, keys []string) bool {
	h.mu.Lock()
	sub, ok := h.subs[subID]
	if !ok || sub.user.Username != username {
		h.mu.Unlock()
		return false
	}
	next := watchSet(keys)
	var added []string
	for key := range next {
		if !sub.watch[key] && watchVisible(sub.user, key) {
			added = append(added, key)
		}
	}
	sub.watch = next
	if len(added) > 0 {
		for _, ev := range h.backlogLocked(added) {
			select {
			case sub.ch <- ev:
			default:
			}
		}
	}
	hooks := h.onWatch
	h.mu.Unlock()
	runHooks(hooks)
	return true
}

// Watched 返回全部订阅者关注范围的并集（仅含该用户可读的键）。
func (h *EventHub) Watched() map[string]bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := map[string]bool{}
	for _, sub := range h.subs {
		for k := range sub.watch {
			if watchVisible(sub.user, k) {
				out[k] = true
			}
		}
	}
	return out
}

// OnWatchChange 登记关注范围变化回调（订阅/退订/SetWatch 后调用，不持锁）。
func (h *EventHub) OnWatchChange(fn func()) {
	h.mu.Lock()
	h.onWatch = append(h.onWatch, fn)
	h.mu.Unlock()
}

// RefreshUser 在账户变更后替换订阅者持有的用户快照；禁用的账户直接断开。
func (h *EventHub) RefreshUser(u webauth.User) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, sub := range h.subs {
		if sub.user.Username != u.Username {
			continue
		}
		if u.Disabled {
			delete(h.subs, id)
			close(sub.ch)
			continue
		}
		sub.user = u
	}
}

// DropUser 断开某用户的全部连接（会话吊销后）。
func (h *EventHub) DropUser(username string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, sub := range h.subs {
		if sub.user.Username == username {
			delete(h.subs, id)
			close(sub.ch)
		}
	}
}

// close 断开全部连接（网关关闭时）。
func (h *EventHub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, sub := range h.subs {
		delete(h.subs, id)
		close(sub.ch)
	}
}

func runHooks(hooks []func()) {
	for _, fn := range hooks {
		fn()
	}
}

// watchSet 规范化 "p/s" / "p" 关注键（去空白、去空项、最多两段）。
func watchSet(keys []string) map[string]bool {
	out := map[string]bool{}
	for _, raw := range keys {
		for _, k := range strings.Split(raw, ",") {
			k = strings.Trim(strings.TrimSpace(k), "/")
			if k == "" || strings.Count(k, "/") > 1 {
				continue
			}
			out[k] = true
		}
	}
	return out
}

// watchSetKeys 返回规范化后的关注键列表（续传实况帧用）。
func watchSetKeys(keys []string) []string {
	set := watchSet(keys)
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	return out
}

func watchVisible(user webauth.User, key string) bool {
	if user.Disabled {
		return false
	}
	if strings.Contains(key, "/") {
		return user.CanRead(key)
	}
	return user.CanReadProject(key)
}

// eventResumeID 读续传点：EventSource 自动带回 Last-Event-ID 头；fetch 流
// 客户端还会用 ?lastEventId= 备份一份（某些代理会吃掉自定义头）。
func eventResumeID(r *http.Request) uint64 {
	if raw := r.Header.Get("Last-Event-ID"); raw != "" {
		id, _ := strconv.ParseUint(raw, 10, 64)
		return id
	}
	if raw := r.URL.Query().Get("lastEventId"); raw != "" {
		id, _ := strconv.ParseUint(raw, 10, 64)
		return id
	}
	return 0
}

// serveEvents 处理 GET /api/events。
func (g *Gateway) serveEvents(w http.ResponseWriter, r *http.Request, user webauth.User) {
	g.startWatcher()
	f, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, http.StatusInternalServerError, "không hỗ trợ luồng sự kiện")
		return
	}
	lastID := eventResumeID(r)
	var watch []string
	for _, v := range r.URL.Query()["watch"] {
		watch = append(watch, v)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	hub := g.events()
	sub, replay, stale := hub.subscribe(user, lastID, watch)
	defer hub.unsubscribe(sub)
	out := newDeadlineWriter(w)

	write := func(id uint64, typ string, data any) bool {
		b, err := json.Marshal(data)
		if err != nil {
			b = []byte("{}")
		}
		if id > 0 {
			_, err = fmt.Fprintf(out, "id: %d\nevent: %s\ndata: %s\n\n", id, typ, b)
		} else {
			_, err = fmt.Fprintf(out, "event: %s\ndata: %s\n\n", typ, b)
		}
		f.Flush()
		return err == nil
	}
	if !write(0, "hello", map[string]any{"subscriber": sub.ID}) {
		return
	}
	if stale && !write(0, "reset", map[string]any{}) {
		return
	}
	for _, ev := range replay {
		if !write(ev.ID, ev.Type, ev.Data) {
			return
		}
	}
	// 实况帧不进 ring，重连后由这里重建关注故事的实况状态（只补不擦）。快照由
	// 客户端自己重取。
	keys := watchSetKeys(watch)
	visible := keys[:0]
	for _, key := range keys {
		if watchVisible(user, key) {
			visible = append(visible, key)
		}
	}
	for _, ev := range hub.Backlog(visible) {
		if !write(0, ev.Type, ev.Data) {
			return
		}
	}
	ping := time.NewTicker(eventPing)
	defer ping.Stop()
	for {
		select {
		case ev, ok := <-sub.ch:
			if !ok || !write(ev.ID, ev.Type, ev.Data) {
				return
			}
			if ev.Type == "session.revoked" && !g.eventSessionValid(r) {
				return
			}
		case <-ping.C:
			if !g.eventSessionValid(r) {
				return
			}
			g.jobsHub().prune()
			if _, err := io.WriteString(out, eventPingComment); err != nil {
				return
			}
			f.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (g *Gateway) eventSessionValid(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	_, _, err = g.accounts.Authenticate(cookie.Value)
	return err == nil
}
