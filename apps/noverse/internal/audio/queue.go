package audio

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"
)

// 生成队列：锁定章节就把章节排进队列，后台逐段生成；状态落盘 audio/queue.json，
// 网关重启后接着跑（重启时"rendering" 视为被中断，回到 "queued"）。

// 队列任务状态（10-audio §2 A4）。
const (
	JobQueued    = "queued"
	JobRendering = "rendering"
	JobPaused    = "paused"
	JobError     = "error"
	JobDone      = "done"
	JobCanceled  = "canceled"
)

// Job kind（§AK）：界面区分 "Đang tạo"（create/redo）与 "Đang cập nhật"（update）。
const (
	JobKindCreate = "create"
	JobKindUpdate = "update"
	JobKindRedo   = "redo"
)

// Job 是一个章节的生成任务。
type Job struct {
	ID      string `json:"jobId"`
	Volume  int    `json:"volume"`
	Chapter int    `json:"chapter"`
	State   string `json:"state"`
	Kind    string `json:"kind"`
	// Position 是 queued 任务里的第几位（1 = 下一个跑）；其它状态读出来是 0，
	// JSON 里省略。不持久化：每次 Snapshot 按当前顺序重算。
	Position      int        `json:"position,omitempty"`
	Quality       string     `json:"quality"`
	RerenderRoles []string   `json:"rerenderRoles,omitempty"`
	Step          string     `json:"step,omitempty"`
	NeedsScript   bool       `json:"needsScript,omitempty"`
	SegmentsDone  int        `json:"segmentsDone"`
	Segments      int        `json:"segments"`
	ETASec        float64    `json:"etaSec"`
	Speaker       string     `json:"speaker"`
	CurrentText   string     `json:"currentText,omitempty"`
	SegmentID     string     `json:"segmentId,omitempty"`
	Error         string     `json:"error,omitempty"`
	ErrorAt       *time.Time `json:"errorAt,omitempty"`
	Percent       float64    `json:"percent"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	// WaitingMachine đúng khi chương tạo giọng đang xếp hàng vì chưa có máy đọc
	// nào sẵn sàng (10-audio.md §8.3); lớp web đặt cờ khi phục vụ.
	WaitingMachine bool `json:"waitingMachine,omitempty"`
}

// Queue 是 audio/queue.json 的内存 + 落盘视图。
type Queue struct {
	mu   sync.Mutex
	path string
	jobs []Job
}

// LoadQueue 读取队列文件；文件不存在返回空队列。
// 重启后"rendering" 任务回到 "queued"——进程已经没了，标成在跑会永远卡住。
func LoadQueue(path string) (*Queue, error) {
	q := &Queue{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return q, nil
	}
	if err != nil {
		return nil, err
	}
	var stored struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, err
	}
	for _, j := range stored.Jobs {
		if j.State == JobRendering {
			j.State = JobQueued
		}
		q.jobs = append(q.jobs, j)
	}
	return q, nil
}

// Save 落盘（调用方不用持锁）。
func (q *Queue) Save() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.saveLocked()
}

func (q *Queue) saveLocked() error {
	data, err := json.MarshalIndent(struct {
		Jobs []Job `json:"jobs"`
	}{Jobs: q.jobs}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(q.path, data)
}

// Snapshot 返回队列副本（API 输出用）：补 Kind 缺省值，并按当前等待顺序算
// Position。Position 在"同一 tập"里从 1 数（界面看到的都是单 tập：sheet、
// dải chương、timeline），queued 才有；其它状态为 0，JSON 省略。
func (q *Queue) Snapshot() []Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Job, len(q.jobs))
	copy(out, q.jobs)
	waiting := map[int]int{}
	for i := range out {
		if out[i].Kind == "" {
			out[i].Kind = JobKindCreate
		}
		out[i].Position = 0
		if out[i].State == JobQueued {
			waiting[out[i].Volume]++
			out[i].Position = waiting[out[i].Volume]
		}
	}
	return out
}

// Enqueue 追加或重置一个章节任务，返回任务 id。
// 同章已有未完成任务时复用其 id（重复锁定不会排出两条）。
func (q *Queue) Enqueue(volume, chapter int) (string, Job) {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	for i, j := range q.jobs {
		if j.Volume != volume || j.Chapter != chapter {
			continue
		}
		if j.State == JobDone || j.State == JobCanceled {
			continue
		}
		q.jobs[i].State = JobQueued
		q.jobs[i].UpdatedAt = now
		q.jobs[i].Error = ""
		q.jobs[i].ErrorAt = nil
		_ = q.saveLocked()
		return q.jobs[i].ID, q.jobs[i]
	}
	job := Job{
		ID:        newJobID(now),
		Volume:    volume,
		Chapter:   chapter,
		State:     JobQueued,
		Kind:      JobKindCreate,
		CreatedAt: now,
		UpdatedAt: now,
	}
	q.jobs = append(q.jobs, job)
	_ = q.saveLocked()
	return job.ID, job
}

// EnqueueRole appends a selective re-read behind work already in progress.
func (q *Queue) EnqueueRole(volume, chapter int, role string, segments int) (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.jobs {
		j := &q.jobs[i]
		if j.Volume != volume || j.Chapter != chapter || j.State != JobQueued || len(j.RerenderRoles) == 0 {
			continue
		}
		for _, current := range j.RerenderRoles {
			if current == role {
				return j.ID, nil
			}
		}
		j.RerenderRoles = append(j.RerenderRoles, role)
		j.Segments += segments
		return j.ID, q.saveLocked()
	}
	now := time.Now()
	job := Job{ID: newJobID(now), Volume: volume, Chapter: chapter, State: JobQueued, Kind: JobKindUpdate, Segments: segments, RerenderRoles: []string{role}, CreatedAt: now, UpdatedAt: now}
	q.jobs = append(q.jobs, job)
	return job.ID, q.saveLocked()
}

// Update 用回调修改任务并落盘；任务不存在返回 false。
func (q *Queue) Update(id string, fn func(*Job)) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.jobs {
		if q.jobs[i].ID != id {
			continue
		}
		fn(&q.jobs[i])
		q.jobs[i].UpdatedAt = time.Now()
		_ = q.saveLocked()
		return true
	}
	return false
}

// Find 返回任务副本（Kind 缺省按 create 补）。
func (q *Queue) Find(id string) (Job, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, j := range q.jobs {
		if j.ID == id {
			if j.Kind == "" {
				j.Kind = JobKindCreate
			}
			return j, true
		}
	}
	return Job{}, false
}

// NextQueued 返回下一个可执行的任务（按队列顺序，"Làm trước" 排最前）。
func (q *Queue) NextQueued() (Job, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, j := range q.jobs {
		if j.State == JobQueued {
			if j.Kind == "" {
				j.Kind = JobKindCreate
			}
			return j, true
		}
	}
	return Job{}, false
}

// Prioritize moves a waiting chapter ahead of other waiting chapters, without
// interrupting the segment currently being synthesized.
func (q *Queue) Prioritize(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.jobs {
		if q.jobs[i].ID != id {
			continue
		}
		if q.jobs[i].State != JobQueued {
			return conflict("Chỉ làm trước được chương đang chờ.")
		}
		job := q.jobs[i]
		copy(q.jobs[1:i+1], q.jobs[:i])
		q.jobs[0] = job
		return q.saveLocked()
	}
	return ErrNotFound
}

// Remove 删除一个任务（测试与"取消后清空"用）。
func (q *Queue) Remove(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := q.jobs[:0]
	for _, j := range q.jobs {
		if j.ID != id {
			out = append(out, j)
		}
	}
	q.jobs = out
	_ = q.saveLocked()
}

// newJobID 生成任务 id：时间戳 + 序号，天然单调、日志好读。
func newJobID(t time.Time) string {
	return t.Format("20060102-150405.000000")
}

// 事件：SSE `S/audio/events` 与"tiến độ qua SSE truyện"共用的载荷。
// Category 固定 "AUDIO"（10-audio §4 GAP-17）。

// 事件类型。
const (
	EventRender  = "audio.render"
	EventCompose = "audio.compose"
	EventQueue   = "audio.queue"
	EventExport  = "audio.export"
)

// EventCategory 是所有音频事件的 category 值。
const EventCategory = "AUDIO"

// Event 是一条进度事件。
type Event struct {
	Type     string         `json:"type"`
	Category string         `json:"category"`
	Data     map[string]any `json:"data"`
}

// NewEvent 构造一条 category=AUDIO 的事件。
func NewEvent(kind string, data map[string]any) Event {
	if data == nil {
		data = map[string]any{}
	}
	return Event{Type: kind, Category: EventCategory, Data: data}
}

// Hub 是最多 N 个 SSE 订阅者的广播中心（慢订阅者丢事件而不是阻塞生成）。
type Hub struct {
	mu     sync.Mutex
	next   int
	subs   map[int]chan Event
	closed bool
}

// NewHub 创建广播中心。
func NewHub() *Hub { return &Hub{subs: map[int]chan Event{}} }

// Subscribe 返回事件通道；ctx 结束后自动退订。
func (h *Hub) Subscribe(ctx context.Context) <-chan Event {
	ch := make(chan Event, 32)
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		close(ch)
		return ch
	}
	id := h.next
	h.next++
	h.subs[id] = ch
	h.mu.Unlock()
	go func() {
		<-ctx.Done()
		h.mu.Lock()
		if c, ok := h.subs[id]; ok {
			delete(h.subs, id)
			close(c)
		}
		h.mu.Unlock()
	}()
	return ch
}

// Publish 广播事件；订阅者缓冲满时丢弃该条。
func (h *Hub) Publish(ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Close 关闭全部订阅（网关退出）。
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for id, ch := range h.subs {
		delete(h.subs, id)
		close(ch)
	}
}
