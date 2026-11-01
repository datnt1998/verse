package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/platform/identity"
)

// ── GAP-26 việc chạy nền ──
//
// 网关汇总所有故事的后台任务：每个存活 worker 的 /api/events + 网关级
// JobsProvider。对外只有 GET /api/events 全局事件流与 /api/jobs 列表/控制。

// 任务状态（14-jobs-dock §1.7）。
const (
	JobRunning = "running"
	JobPausedS = "paused"
	JobWaiting = "waiting"
	JobFailed  = "failed"
	JobDoneS   = "done"
)

// jobRetention 是已完成任务在内存中保留的时长。
const jobRetention = 10 * time.Minute

// Job 是 dock 的一行；ID 稳定为 "<p>/<s>/<kind>/<n>"（音频失败章可再带一段）。
type Job struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	Status     string     `json:"status"`
	Progress   *float64   `json:"progress"`
	Step       string     `json:"step,omitempty"`
	Unit       *JobUnit   `json:"unit,omitempty"`
	ETASec     int        `json:"etaSec,omitempty"`
	Error      any        `json:"error,omitempty"` // string 或 §AE 的 JobErrorInfo
	Result     *JobResult `json:"result,omitempty"`
	Href       string     `json:"href"`
	Project    string     `json:"project"`
	Story      string     `json:"story"`
	Agent      string     `json:"agent,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

// JobErrorInfo 是 §AE 的错误结构：export 任务用它带 chapter/cause/retryable；
// 其它任务保持 error 为字符串（FE 两种都认）。
type JobErrorInfo struct {
	Message   string `json:"message"`
	Chapter   *int   `json:"chapter,omitempty"`
	Retryable bool   `json:"retryable"`
	Cause     string `json:"cause,omitempty"`
}

// JobResultFile 是 §AE result.files[] 的一项（音频导出成品）。
type JobResultFile struct {
	Name        string  `json:"name"`
	SizeBytes   int64   `json:"sizeBytes"`
	URL         string  `json:"url,omitempty"`
	Path        string  `json:"path,omitempty"`
	Chapter     int     `json:"chapter,omitempty"`
	DurationSec float64 `json:"durationSec,omitempty"`
}

// JobUnit 是 "Chương 5/12" 这类计数。
type JobUnit struct {
	Done  int    `json:"done"`
	Total int    `json:"total"`
	Label string `json:"label"`
}

// Files/ZipURL 对应 §AE：result:{files:[{name,sizeBytes,url}], zipUrl?}。

// JobResult 是任务完成（或等待）时的结果卡片。
// Files/ZipURL 对应 §AE：result:{files:[{name,sizeBytes,url}], zipUrl?}。
type JobResult struct {
	Title   string          `json:"title"`
	Sub     string          `json:"sub"`
	Actions []JobAction     `json:"actions"`
	Data    any             `json:"data,omitempty"`
	Files   []JobResultFile `json:"files,omitempty"`
	ZipURL  string          `json:"zipUrl,omitempty"`
}

// JobAction 是结果按钮。Kind：download（Href 为 API 地址）· reveal（POST /api/reveal）· open（Href 为 SPA 路径）。
type JobAction struct {
	ID      string `json:"id,omitempty"`
	Label   string `json:"label"`
	Primary bool   `json:"primary,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Icon    string `json:"icon,omitempty"`
	Path    string `json:"path,omitempty"`
	Href    string `json:"href,omitempty"`
}

// JobChange 是来源推给汇总中心的一次变化；Removed 表示该任务应从列表消失。
type JobChange struct {
	Job     Job
	Removed bool
}

// JobsProvider 是网关级任务来源（例如音频生成队列）。
type JobsProvider interface {
	// Kind 是该来源负责的任务种类（ID 第三段），例如 "audio"。
	Kind() string
	// Jobs 返回全部故事的当前任务（汇总中心按用户权限过滤）。
	Jobs() []Job
	// Watch 阻塞到 ctx 结束，每次变化调用 emit。
	Watch(ctx context.Context, emit func(JobChange))
	// Control 执行 pause|resume|cancel|retry；不认识的任务返回 errJobNotFound，
	// 不支持的操作返回 errJobUnsupported。
	Control(ctx context.Context, id, op string) error
}

// JobsBodyProvider 是可选扩展：retry 带请求体（§AE 的 {intro:false} = Ghép không nhạc）。
// 实现了它的 provider 在收到 body 时被调用 ControlBody，否则回退 Control。
type JobsBodyProvider interface {
	ControlBody(ctx context.Context, id, op string, body map[string]any) error
}

var (
	errJobNotFound    = errors.New("Không tìm thấy việc này")
	errJobUnsupported = errors.New("Việc này không hỗ trợ thao tác đó")
)

// jobOps 是 POST /api/jobs/:id/:op 允许的操作。
var jobOps = map[string]bool{"pause": true, "resume": true, "cancel": true, "retry": true}

// jobEventName 依据前后状态给出 SSE 事件名。
func jobEventName(prev Job, existed bool, next Job) string {
	switch {
	case !existed:
		if next.Status == JobWaiting {
			return "job.waiting"
		}
		if next.Status == JobFailed {
			return "job.failed"
		}
		if next.Status == JobDoneS {
			return "job.done"
		}
		return "job.created"
	case prev.Status != next.Status && next.Status == JobWaiting:
		return "job.waiting"
	case prev.Status != next.Status && next.Status == JobFailed:
		return "job.failed"
	case prev.Status != next.Status && next.Status == JobDoneS:
		return "job.done"
	default:
		return "job.progress"
	}
}

// jobsHub 是网关内存中的任务汇总表；变化经全局事件中心（events.go）广播。
type jobsHub struct {
	mu             sync.Mutex
	jobs           map[string]Job
	pub            *EventHub
	providers      map[string]JobsProvider
	watching       map[string]*launchedWorker
	ctx            context.Context
	cancel         context.CancelFunc
	now            func() time.Time
	seq            uint64
	controls       map[string]func(context.Context, string) error
	dismissed      map[string]time.Time
	deletedStories map[string]bool
}

func newJobsHub(pub *EventHub) *jobsHub {
	ctx, cancel := context.WithCancel(context.Background())
	return &jobsHub{
		jobs: map[string]Job{}, pub: pub, providers: map[string]JobsProvider{},
		watching: map[string]*launchedWorker{}, ctx: ctx, cancel: cancel, now: time.Now,
	}
}

// jobsHub 惰性创建（测试里有直接 &Gateway{} 的构造）。
func (g *Gateway) jobsHub() *jobsHub {
	g.jobsOnce.Do(func() {
		if g.jobs == nil {
			g.jobs = newJobsHub(g.events())
		}
	})
	return g.jobs
}

// addProvider 登记一个网关级来源：先灌入现有任务，再后台监听变化。
func (h *jobsHub) addProvider(p JobsProvider) {
	h.mu.Lock()
	h.providers[p.Kind()] = p
	h.mu.Unlock()
	for _, j := range p.Jobs() {
		h.apply(JobChange{Job: j})
	}
	go p.Watch(h.ctx, h.apply)
}

func (h *jobsHub) close() { h.cancel() }

// prune 清理过期的已完成任务（广播 job.removed）。
func (h *jobsHub) prune() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruneLocked()
}

// apply 合并一次变化并广播给有权限的订阅者。
func (h *jobsHub) apply(c JobChange) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.applyLocked(c)
	h.pruneLocked()
}

// normalizeJobError 统一 Job.Error 的"无错"表示为真正的 nil：
// 改成 any 之后，"" 是任何里包着的非 nil 值（omitempty 会输出 "error":""，
// DeepEqual("","nil") 为假），所以空串/类型化 nil 都在入口和赋值处归一。
func normalizeJobError(j *Job) {
	switch e := j.Error.(type) {
	case string:
		if e == "" {
			j.Error = nil
		}
	case error:
		if e == nil {
			j.Error = nil
		}
	case *JobErrorInfo:
		if e == nil {
			j.Error = nil
		}
	}
}
func (h *jobsHub) applyLocked(c JobChange) {
	normalizeJobError(&c.Job)
	if !c.Removed && h.deletedStories[c.Job.Project+"/"+c.Job.Story] {
		return
	}
	id := c.Job.ID
	prev, existed := h.jobs[id]
	if c.Removed {
		if !existed {
			return
		}
		delete(h.jobs, id)
		delete(h.controls, id)
		h.broadcastLocked(prev, "job.removed", map[string]string{"id": id})
		return
	}
	if until, hidden := h.dismissed[id]; hidden {
		if c.Job.Status == JobDoneS && h.now().Before(until) {
			return
		}
		delete(h.dismissed, id)
	}
	if existed && jobsEqual(prev, c.Job) {
		return
	}
	h.jobs[id] = c.Job
	h.broadcastLocked(c.Job, jobEventName(prev, existed, c.Job), c.Job)
}

func (h *jobsHub) dismiss(job Job) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dismissed == nil {
		h.dismissed = map[string]time.Time{}
	}
	h.dismissed[job.ID] = h.now().Add(jobRetention)
	h.applyLocked(JobChange{Job: job, Removed: true})
}

func jobsEqual(a, b Job) bool {
	return reflect.DeepEqual(a, b)
}

// pruneLocked 清掉保留期已过的已完成任务。
func (h *jobsHub) pruneLocked() {
	cutoff := h.now().Add(-jobRetention)
	for id, j := range h.jobs {
		if j.Status == JobDoneS && j.FinishedAt != nil && j.FinishedAt.Before(cutoff) {
			delete(h.jobs, id)
			delete(h.controls, id)
			h.broadcastLocked(j, "job.removed", map[string]string{"id": id})
		}
	}
	for id, until := range h.dismissed {
		if !h.now().Before(until) {
			delete(h.dismissed, id)
		}
	}
}

func (h *jobsHub) broadcastLocked(j Job, event string, data any) {
	// Việc của máy (Project/Story rỗng) không lọc được bằng quyền đọc truyện nên
	// phải đánh dấu riêng: cùng quyền với GET /api/tts/install và danh sách việc.
	h.pub.Publish(Event{Type: event, Project: j.Project, Story: j.Story, AdminOnly: jobOfMachine(j), Data: data})
}

// list 返回用户可读的任务（开始时间倒序，ID 兜底排序保证稳定）。
func (h *jobsHub) list(user identity.User) []Job {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruneLocked()
	out := make([]Job, 0, len(h.jobs))
	for _, j := range h.jobs {
		if jobOfMachine(j) {
			if canControlMachineJobs(user) {
				out = append(out, j)
			}
			continue
		}
		if user.CanRead(j.Project + "/" + j.Story) {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if !out[a].StartedAt.Equal(out[b].StartedAt) {
			return out[a].StartedAt.After(out[b].StartedAt)
		}
		return out[a].ID < out[b].ID
	})
	return out
}

// jobOfMachine báo việc của máy (cài bộ đọc…) — không thuộc truyện nào nên không
// lọc được bằng quyền đọc truyện.
func jobOfMachine(j Job) bool { return j.Project == "" && j.Story == "" }

// canControlMachineJobs: việc của máy chạm vào máy chủ nên chỉ quản trị viên thấy
// và điều khiển, cùng quyền với POST /api/tts/install.
func canControlMachineJobs(user identity.User) bool {
	return user.Role == identity.RoleAdmin && !user.Disabled
}

func (h *jobsHub) get(id string) (Job, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	j, ok := h.jobs[id]
	return j, ok
}

// resetStory 用 worker 的完整快照替换该故事由 worker 产生的任务（网关级来源的不动）。
func (h *jobsHub) resetStory(project, story string, jobs []Job) {
	h.mu.Lock()
	defer h.mu.Unlock()
	keep := map[string]bool{}
	for _, j := range jobs {
		keep[j.ID] = true
		h.applyLocked(JobChange{Job: j})
	}
	for id, j := range h.jobs {
		if j.Project == project && j.Story == story && !keep[id] && j.Status != JobDoneS && h.providers[j.Kind] == nil && h.controls[id] == nil {
			h.applyLocked(JobChange{Job: j, Removed: true})
		}
	}
	h.pruneLocked()
}

// dropStory 在 worker 退出时撤下其未完成任务（已完成的结果保留到过期）。
func (h *jobsHub) dropStory(project, story string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, j := range h.jobs {
		if j.Project == project && j.Story == story && j.Status != JobDoneS && h.providers[j.Kind] == nil && h.controls[j.ID] == nil {
			h.applyLocked(JobChange{Job: j, Removed: true})
		}
	}
}

// ── worker → 网关 ──

// splitStoryKey 把槽位键 "p/s" 拆开；草稿槽（draft//…）等其它键返回 ok=false。
func splitStoryKey(key string) (project, story string, ok bool) {
	if strings.HasPrefix(key, "draft//") {
		return "", "", false
	}
	project, story, ok = strings.Cut(key, "/")
	if !ok || project == "" || story == "" || strings.Contains(story, "/") {
		return "", "", false
	}
	return project, story, true
}

// storyAPIBase / storyRouteBase 是故事的 API 前缀与 SPA 路由前缀。
func storyAPIBase(project, story string) string {
	return "/api/projects/" + url.PathEscape(project) + "/stories/" + url.PathEscape(story)
}

func storyRouteBase(project, story string) string {
	return "/p/" + url.PathEscape(project) + "/s/" + url.PathEscape(story)
}

// qualifyJob 把 worker 本地任务（ID "<kind>/<n>"、相对 href）补成全局形态。
func qualifyJob(project, story string, j Job) Job {
	j.ID = project + "/" + story + "/" + j.ID
	j.Project, j.Story = project, story
	j.Href = qualifyHref(storyRouteBase(project, story), j.Href)
	if j.Result != nil {
		res := *j.Result
		res.Actions = append([]JobAction(nil), res.Actions...)
		for i, a := range res.Actions {
			base := storyRouteBase(project, story)
			if a.Kind == "download" {
				base = storyAPIBase(project, story)
			}
			res.Actions[i].Href = qualifyHref(base, a.Href)
		}
		j.Result = &res
	}
	return j
}

func qualifyHref(base, rel string) string {
	if strings.HasPrefix(rel, "/") {
		return rel
	}
	if rel == "" {
		return base
	}
	if strings.HasPrefix(rel, "?") {
		return base + rel
	}
	return base + "/" + rel
}

// watchWorkerJobs 在 worker 启动后挂上任务监听（同一 worker 只挂一次）。
func (g *Gateway) watchWorkerJobs(key string, worker *launchedWorker) {
	project, story, ok := splitStoryKey(key)
	if !ok || worker == nil {
		return
	}
	hub := g.jobsHub()
	hub.mu.Lock()
	if hub.watching[key] == worker {
		hub.mu.Unlock()
		return
	}
	hub.watching[key] = worker
	hub.mu.Unlock()
	go func() {
		defer func() {
			hub.mu.Lock()
			owns := hub.watching[key] == worker
			if owns {
				delete(hub.watching, key)
			}
			hub.mu.Unlock()
			if owns {
				hub.dropStory(project, story)
			}
		}()
		for {
			g.streamWorkerJobs(hub.ctx, project, story, worker)
			if workerDead(worker) || hub.ctx.Err() != nil {
				return
			}
			select {
			case <-worker.done:
				return
			case <-hub.ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()
}

// streamWorkerJobs 读取 worker 的统一事件流。
func (g *Gateway) streamWorkerJobs(ctx context.Context, project, story string, worker *launchedWorker) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-worker.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+worker.addr+"/api/events", nil)
	if err != nil {
		return
	}
	req.Header.Set("X-Ainovel-Worker-Token", g.workerToken)
	client := &http.Client{}
	if rt := g.workerTransport(); rt != nil {
		client.Transport = rt
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	live := newLiveMux(g, project, story)
	readJobStream(resp.Body, func(event string, data []byte) {
		live.frame(event, data)
		g.publishWorkerChange(project+"/"+story, event, data)
		hub := g.jobsHub()
		switch event {
		case "jobs.reset":
			var v struct {
				Jobs []Job `json:"jobs"`
			}
			if json.Unmarshal(data, &v) != nil {
				return
			}
			out := make([]Job, 0, len(v.Jobs))
			for _, j := range v.Jobs {
				out = append(out, qualifyJob(project, story, j))
			}
			hub.resetStory(project, story, out)
		case "job.removed":
			var v struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(data, &v) != nil {
				return
			}
			hub.apply(JobChange{Job: Job{ID: project + "/" + story + "/" + v.ID}, Removed: true})
		default:
			if !strings.HasPrefix(event, "job.") {
				return
			}
			var j Job
			if json.Unmarshal(data, &j) != nil {
				return
			}
			hub.apply(JobChange{Job: qualifyJob(project, story, j)})
		}
	})
}

// readJobStream 解析 SSE（event:/data: 行，空行分帧，忽略注释行）。
func readJobStream(r io.Reader, handle func(event string, data []byte)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	event, data := "", []string{}
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if event != "" && len(data) > 0 {
				handle(event, []byte(strings.Join(data, "\n")))
			}
			event, data = "", data[:0]
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
}

// ── HTTP ──

// serveJobs 处理 /api/jobs*；返回 false 表示路径不归它管。
func (g *Gateway) serveJobs(w http.ResponseWriter, r *http.Request, user identity.User) bool {
	path := r.URL.Path
	if path != "/api/jobs" && !strings.HasPrefix(path, "/api/jobs/") {
		return false
	}
	hub := g.jobsHub()
	switch {
	case path == "/api/jobs" && r.Method == http.MethodGet:
		g.refreshJobs(r.Context(), user)
		writeJSON(w, http.StatusOK, map[string]any{"jobs": hub.list(user)})
	case r.Method == http.MethodGet:
		g.refreshJobs(r.Context(), user)
		g.getJobDetail(w, r, user.CanRead, strings.TrimPrefix(path, "/api/jobs/"))
	case r.Method == http.MethodPost:
		g.controlJob(w, r, user, strings.TrimPrefix(path, "/api/jobs/"))
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
	return true
}

// controlJob 处理 POST /api/jobs/<id>/<op>：路由到来源（story worker / gateway provider）。
func (g *Gateway) controlJob(w http.ResponseWriter, r *http.Request, user identity.User, rest string) {
	segs := strings.Split(rest, "/")
	if len(segs) < 3 {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	op := segs[len(segs)-1]
	if !jobOps[op] {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	// r.URL.Path đã được net/http giải mã một lần; giải mã lần nữa biến %252F
	// thành dấu "/" ngay trong một đoạn và làm id khác với đường dẫn được kiểm
	// quyền. Đoạn rỗng hay ".." cũng không bao giờ là id hợp lệ.
	for _, s := range segs {
		if s == "" || s == "." || s == ".." || strings.Contains(s, "/") {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return
		}
	}
	id := strings.Join(segs[:len(segs)-1], "/")
	hub := g.jobsHub()
	job, known := hub.get(id)
	machine := known && jobOfMachine(job)
	storyID := ""
	if machine {
		// Việc của máy không thuộc truyện nào: quyền theo máy, không theo truyện.
		if !canControlMachineJobs(user) {
			jsonError(w, http.StatusNotFound, errJobNotFound.Error())
			return
		}
	} else {
		if len(segs) < 5 {
			jsonError(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		storyID = segs[0] + "/" + segs[1]
		// 与故事路由同一隐藏规则：读不到或写不了都回 404，不暴露存在性。
		if !user.CanRead(storyID) || !user.CanWrite(storyID) {
			jsonError(w, http.StatusNotFound, errJobNotFound.Error())
			return
		}
	}
	// 来源按 ID 的 kind 段优先（"p/s/audio/1/export" 的 kind=export 但归属 audio
	// provider——§AE），查不到再退回任务自己的 kind。
	kind := segs[2]
	hub.mu.Lock()
	provider := hub.providers[kind]
	if provider == nil && known {
		provider = hub.providers[job.Kind]
	}
	control := hub.controls[id]
	hub.mu.Unlock()
	// retry 允许带 JSON body（§AE：{intro:false} → Ghép không nhạc）。
	var body map[string]any
	if op == "retry" && r.Body != nil {
		defer r.Body.Close()
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body)
	}
	// 已完成任务的“huỷ” = 从列表移除（Bỏ khỏi danh sách）。
	if known && job.Status == JobDoneS && op == "cancel" {
		switch {
		case control != nil:
			_ = control(r.Context(), op)
		case provider != nil:
			_ = provider.Control(r.Context(), id, op)
		case !machine:
			_ = g.forwardJobControl(r.Context(), storyID, strings.Join(segs[2:len(segs)-1], "/"), op)
		}
		hub.dismiss(job)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	var err error
	switch {
	case control != nil:
		err = control(r.Context(), op)
	case provider != nil:
		if bp, ok := provider.(JobsBodyProvider); ok {
			err = bp.ControlBody(r.Context(), id, op, body)
		} else {
			err = provider.Control(r.Context(), id, op)
		}
	case machine:
		err = errJobNotFound
	default:
		err = g.forwardJobControl(r.Context(), storyID, strings.Join(segs[2:len(segs)-1], "/"), op)
	}
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case errors.Is(err, errJobNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	default:
		jsonError(w, http.StatusConflict, err.Error())
	}
}

// forwardJobControl 把操作转给故事 worker（仅在 worker 已存活时；不为此启动 worker）。
func (g *Gateway) forwardJobControl(ctx context.Context, storyID, localID, op string) error {
	g.mu.Lock()
	slot := g.workers[storyID]
	g.mu.Unlock()
	if slot == nil {
		return errJobNotFound
	}
	slot.mu.Lock()
	worker := slot.worker
	slot.mu.Unlock()
	if worker == nil || workerDead(worker) {
		return errJobNotFound
	}
	status, body, err := g.workerCall(ctx, worker, http.MethodPost, "jobs/"+localID+"/"+op, map[string]any{})
	if err != nil {
		return err
	}
	switch {
	case status == http.StatusOK:
		return nil
	case status == http.StatusNotFound:
		return errJobNotFound
	default:
		return errors.New(workerError(status, body))
	}
}
