package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/platform/events"
	"github.com/datnt1998/verse/platform/identity"
)

// ── 全局事件流 GET /api/events（16-feedback-refresh §2/§4）──
//
// 网关唯一的 SSE：job.*（jobs.go）与各类失效事件（library/project/story/config/
// account/session）共用。续传环形缓冲、递增 id、hello/reset/心跳与写截止都在
// platform/events；本文件只保留 Noverse 的三块私有逻辑：
//   - 可见性：User/Story/Project/AdminsToo 按 identity.User 过滤；
//   - 关注集（watch）：订阅者声明正在看的 "p/s"/"p"，文件监视据此只扫有人看的故事；
//   - 实况帧：worker/audio 的高频帧走瞬时广播，不进续传环。

// liveDataType 标记实况帧类型（/api/events 上唯一的 event 名）。
const liveDataType = "live"

// liveFrame 是 worker/audio 实况帧在全局流里的信封；kind 与 worker SSE 的
// event 名一致（snapshot · event · delta · clear · cocreate · job · done…）。
// 走瞬时广播：不进 ring、不带 id，重连后靠快照 + 正文尾巴重建。
type liveFrame struct {
	Project string          `json:"project"`
	Story   string          `json:"story"`
	Kind    string          `json:"kind"`
	Data    json.RawMessage `json:"data"`
}

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

func (e Event) visibleTo(u identity.User) bool {
	switch {
	case u.Disabled:
		return false
	case e.User != "":
		return u.Username == e.User || (e.AdminsToo && u.Role == identity.RoleAdmin)
	case e.Story != "":
		return u.CanRead(e.Project + "/" + e.Story)
	case e.Project != "":
		return u.CanReadProject(e.Project)
	default:
		return true
	}
}

// toWire 把应用事件装进平台帧：整个 Event 放进 Meta 供可见性过滤与回放
// 转换用，ID 由 Publish 在平台帧上分配（发布侧恒为 0；回放帧带原 id）。
func toWire(ev Event) events.Event {
	return events.Event{ID: ev.ID, Type: ev.Type, Data: ev.Data, Meta: ev}
}

// fromWire 把平台帧还原成应用事件（回放/发布钩子用）；线上帧的 Meta 恒为
// toWire 放回的 Event。
func fromWire(pe events.Event) Event {
	ev, _ := pe.Meta.(Event)
	ev.ID = pe.ID
	ev.Type = pe.Type
	ev.Data = pe.Data
	return ev
}

// EventSub 是一个 /api/events 连接。Tag 里是客户端声明正在看的范围
// （"p/s" 或 "p"，?watch= 与 SetWatch 更新，存成 map[string]bool），供文件
// 监视只扫有人看的故事；Tag 只在 hub 锁内被 WithSub/ForEachSub 读写。
type EventSub struct {
	*events.Sub
}

// EventHub 是全局事件中心：platform/events 的通用 hub 外面包一层 Noverse
// 的可见性、关注集与实况帧存储。
type EventHub struct {
	inner *events.Hub

	hookMu  sync.Mutex
	onWatch []func()

	liveMu     sync.Mutex
	streams    map[string]string            // "p/s" → 流式正文尾巴（瞬时，不进 ring）
	liveEvents map[string][]json.RawMessage // "p/s" → 最近实况 event 帧（补发用）
}

func newEventHub() *EventHub {
	return &EventHub{inner: events.New(func(p any, pe events.Event) bool {
		u, _ := p.(identity.User)
		ev, ok := pe.Meta.(Event)
		if !ok {
			return false
		}
		return ev.visibleTo(u)
	})}
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
	return h.inner.Publish(toWire(ev))
}

// Broadcast 扇出一条不进续传缓冲、不分配 id 的瞬时事件：用于 worker/audio
// 的高频实况帧（delta 每秒几十帧），它们不该把 500 条的续传环挤掉。
func (h *EventHub) Broadcast(ev Event) {
	h.inner.Broadcast(toWire(ev))
}

// OnPublish 登记每条事件发布后的回调（锁外调用），恢复旧 onEvent 钩子的语义。
func (h *EventHub) OnPublish(fn func(Event)) {
	h.inner.OnPublish(func(pe events.Event) { fn(fromWire(pe)) })
}

// Backlog 返回关注故事的实况补发帧（最近事件 · clear · 正文尾巴），与旧
// worker 直连流的 reset→replay→clear→delta 等价：只补不擦，客户端按 id 合并。
func (h *EventHub) Backlog(keys []string) []Event {
	h.liveMu.Lock()
	defer h.liveMu.Unlock()
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
		if list := h.liveEvents[key]; len(list) > 0 {
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
	h.liveMu.Lock()
	if h.streams == nil {
		h.streams = map[string]string{}
	}
	h.streams[key] = text
	h.liveMu.Unlock()
}

// liveEventCap 是每故事为“新连接补发”保留的实况事件条数（与旧 worker 直连流的
// ring 语义一致，只是不进全局续传环）。
const liveEventCap = 200

// AppendLiveEvent 记录一条实况 event 帧，供新标签页/重连补发。
func (h *EventHub) AppendLiveEvent(key string, frame json.RawMessage) {
	h.liveMu.Lock()
	if h.liveEvents == nil {
		h.liveEvents = map[string][]json.RawMessage{}
	}
	list := append(h.liveEvents[key], frame)
	if len(list) > liveEventCap {
		list = list[len(list)-liveEventCap:]
	}
	h.liveEvents[key] = list
	h.liveMu.Unlock()
}

// ResetLiveEvents 清空某故事的实况事件（worker 发 reset，说明它换了一轮）。
func (h *EventHub) ResetLiveEvents(key string) {
	h.liveMu.Lock()
	delete(h.liveEvents, key)
	h.liveMu.Unlock()
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

// subWatch 读订阅者的关注集（Tag 里存 map[string]bool，未声明时是 nil）。
func subWatch(s *events.Sub) map[string]bool {
	watch, _ := s.Tag.(map[string]bool)
	return watch
}

func (h *EventHub) subscribe(user identity.User, lastID uint64, watch []string) (sub *EventSub, replay []Event, stale bool) {
	inner, wireReplay, stale := h.inner.Subscribe(user, lastID)
	sub = &EventSub{Sub: inner}
	if ws := watchSet(watch); len(ws) > 0 {
		h.inner.WithSub(inner.ID, func(s *events.Sub) bool {
			s.Tag = ws
			return true
		})
		runHooks(h.watchHooks())
	}
	for _, ev := range wireReplay {
		replay = append(replay, fromWire(ev))
	}
	return sub, replay, stale
}

func (h *EventHub) unsubscribe(sub *EventSub) {
	hadWatch := h.inner.WithSub(sub.ID, func(s *events.Sub) bool {
		return len(subWatch(s)) > 0
	})
	h.inner.Unsubscribe(sub.Sub)
	if hadWatch {
		runHooks(h.watchHooks())
	}
}

// SetWatch 替换订阅者的关注范围；订阅者不存在或不属于该用户返回 false。
// 新加的故事立刻补发实况尾部（最近事件 + 正文），否则要等到下一次重连才看到。
// 可见性检查、实况尾部抓取与入队都在 hub 锁内作为一个批次完成（旧实现的
// 语义）：RefreshUser 在两步之间降权不会漏过被禁的帧，worker 的瞬时帧也插
// 不进 "events → clear → delta" 的顺序。锁序固定为 hub → liveMu：Backlog
// 只在持有 hub 锁的回调里取 liveMu，不会反序。
func (h *EventHub) SetWatch(subID, username string, keys []string) bool {
	ok := h.inner.Backfill(subID, func(s *events.Sub) ([]events.Event, bool) {
		u, _ := s.Principal().(identity.User)
		if u.Username != username {
			return nil, false
		}
		next := watchSet(keys)
		old := subWatch(s)
		var added []string
		for key := range next {
			if !old[key] && watchVisible(u, key) {
				added = append(added, key)
			}
		}
		s.Tag = next
		var batch []events.Event
		for _, ev := range h.Backlog(added) {
			batch = append(batch, toWire(ev))
		}
		return batch, true
	})
	if !ok {
		return false
	}
	runHooks(h.watchHooks())
	return true
}

// RemoveWatch 从所有订阅者的关注集里删掉一个键（故事被删除后），并触发关注
// 变化回调——与旧的 removeStoryWatch 等价。
func (h *EventHub) RemoveWatch(key string) {
	h.inner.ForEachSub(func(s *events.Sub) {
		delete(subWatch(s), key)
	})
	runHooks(h.watchHooks())
}

// Watched 返回全部订阅者关注范围的并集（仅含该用户可读的键）。
func (h *EventHub) Watched() map[string]bool {
	out := map[string]bool{}
	h.inner.ForEachSub(func(s *events.Sub) {
		u, _ := s.Principal().(identity.User)
		for key := range subWatch(s) {
			if watchVisible(u, key) {
				out[key] = true
			}
		}
	})
	return out
}

// OnWatchChange 登记关注范围变化回调（订阅/退订/SetWatch 后调用，不持锁）。
func (h *EventHub) OnWatchChange(fn func()) {
	h.hookMu.Lock()
	h.onWatch = append(h.onWatch, fn)
	h.hookMu.Unlock()
}

func (h *EventHub) watchHooks() []func() {
	h.hookMu.Lock()
	defer h.hookMu.Unlock()
	return append([]func(){}, h.onWatch...)
}

// RefreshUser 在账户变更后替换订阅者持有的用户快照；禁用的账户直接断开。
func (h *EventHub) RefreshUser(u identity.User) {
	h.inner.Update(func(p any) (any, bool) {
		cur, _ := p.(identity.User)
		if cur.Username != u.Username {
			return p, true
		}
		if u.Disabled {
			return p, false
		}
		return u, true
	})
}

// DropUser 断开某用户的全部连接（会话吊销后）。
func (h *EventHub) DropUser(username string) {
	h.inner.Update(func(p any) (any, bool) {
		u, _ := p.(identity.User)
		return p, u.Username != username
	})
}

// close 断开全部连接（网关关闭时）。
func (h *EventHub) close() { h.inner.Close() }

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

func watchVisible(user identity.User, key string) bool {
	if user.Disabled {
		return false
	}
	if strings.Contains(key, "/") {
		return user.CanRead(key)
	}
	return user.CanReadProject(key)
}

// serveEvents 处理 GET /api/events。
func (g *Gateway) serveEvents(w http.ResponseWriter, r *http.Request, user identity.User) {
	g.startWatcher()
	hub := g.events()
	var watch []string
	for _, v := range r.URL.Query()["watch"] {
		watch = append(watch, v)
	}
	sub, replay, stale := hub.subscribe(user, events.ResumeID(r), watch)
	defer hub.unsubscribe(sub)

	var wire []events.Event
	for _, ev := range replay {
		wire = append(wire, toWire(ev))
	}
	hooks := events.Hooks{
		// 实况帧不进 ring，重连后由这里重建关注故事的实况状态（只补不擦）。快照由
		// 客户端自己重取。
		Preamble: func(write func(events.Event) bool) {
			keys := watchSetKeys(watch)
			visible := keys[:0]
			for _, key := range keys {
				if watchVisible(user, key) {
					visible = append(visible, key)
				}
			}
			for _, ev := range hub.Backlog(visible) {
				if !write(toWire(ev)) {
					return
				}
			}
		},
		AfterFrame: func(ev events.Event) bool {
			return ev.Type != "session.revoked" || g.eventSessionValid(r)
		},
		OnTick: func() bool {
			if !g.eventSessionValid(r) {
				return false
			}
			g.jobsHub().prune()
			return true
		},
	}
	if err := hub.inner.Serve(w, r, sub.Sub, wire, stale, hooks); err != nil {
		jsonError(w, http.StatusInternalServerError, "không hỗ trợ luồng sự kiện")
	}
}

func (g *Gateway) eventSessionValid(r *http.Request) bool {
	cookie, err := r.Cookie(g.cookiePolicy().Name)
	if err != nil {
		return false
	}
	user, _, err := g.accounts.Authenticate(cookie.Value)
	// The recheck enforces the same boundary as the request path: the
	// session must still authenticate AND still carry the Noverse grant.
	return err == nil && appAccessGranted(user.Grants)
}
