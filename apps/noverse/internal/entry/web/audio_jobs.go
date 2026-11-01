package web

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
)

type audioJobsProvider struct{ g *Gateway }

func newAudioJobsProvider(g *Gateway) JobsProvider { return audioJobsProvider{g: g} }

// machineReady: chương tạo giọng xếp hàng có phải đang chờ máy đọc không.
// Bản cá nhân đọc bằng máy này nên luôn "có máy"; chỉ bản máy chủ mới chờ.
func (p audioJobsProvider) machineReady(ctx context.Context) bool {
	if p.g == nil {
		return true
	}
	return p.g.voiceMachineReady(ctx)
}

func (audioJobsProvider) Kind() string { return "audio" }

func (p audioJobsProvider) Jobs() []Job {
	var out []Job
	machineReady := p.machineReady(context.Background())
	audioServices.Range(func(_, v any) bool {
		out = append(out, audioRows(v.(audioEntry), time.Now(), machineReady)...)
		return true
	})
	return out
}

// Watch 订阅每个服务的事件；新服务出现时接上；行消失时发 Removed。
func (p audioJobsProvider) Watch(ctx context.Context, emit func(JobChange)) {
	changed := make(chan string, 64)
	subscribed := map[*audio.Service]bool{}
	last := map[string]map[string]Job{} // dir → id → 上次发出的行
	subscribe := func() {
		audioServices.Range(func(k, v any) bool {
			entry := v.(audioEntry)
			if subscribed[entry.svc] {
				return true
			}
			subscribed[entry.svc] = true
			dir := k.(string)
			events := entry.svc.Hub().Subscribe(ctx)
			go func() {
				for ev := range events {
					if ev.Type == audio.EventQueue || ev.Type == audio.EventRender || ev.Type == audio.EventExport {
						select {
						case changed <- dir:
						default:
						}
					}
				}
			}()
			select {
			case changed <- dir:
			default:
			}
			return true
		})
	}
	for {
		signal := audioServicesSignal()
		subscribe()
		select {
		case <-ctx.Done():
			return
		case <-signal:
		case dir := <-changed:
			v, ok := audioServices.Load(dir)
			now := map[string]Job{}
			if ok {
				for _, row := range audioRows(v.(audioEntry), time.Now(), p.machineReady(ctx)) {
					now[row.ID] = row
					emit(JobChange{Job: row})
				}
			}
			for id, old := range last[dir] {
				if _, still := now[id]; !still {
					emit(JobChange{Job: old, Removed: true})
				}
			}
			last[dir] = now
		}
	}
}

// Control 处理任务中心的操作。卷行：pause|resume|cancel 作用于整卷；
// 章节出错行：retry 重新排队，cancel = "Bỏ qua chương này"。
// "<p>/<s>/audio/<v>/export" 行：retry|cancel 作用于持久化的导出请求。
func (p audioJobsProvider) Control(ctx context.Context, id, op string) error {
	return p.ControlBody(ctx, id, op, nil)
}

// ControlBody 与 Control 相同，但把 retry 的 JSON body（§AE {intro:false}）
// 传给持久化的导出请求。
func (audioJobsProvider) ControlBody(_ context.Context, id, op string, body map[string]any) error {
	if strings.HasSuffix(id, "/export") {
		entry, volume, chapter, ok := parseAudioJobID(strings.TrimSuffix(id, "/export"))
		if !ok || chapter != 0 {
			return errJobNotFound
		}
		if op != "retry" && op != "cancel" {
			return errJobUnsupported
		}
		return audioJobErr(entry.svc.ControlExportBody(volume, op, body))
	}
	entry, volume, chapter, ok := parseAudioJobID(id)
	if !ok {
		return errJobNotFound
	}
	jobs := entry.svc.Queue()
	if chapter > 0 {
		for _, j := range jobs {
			if j.Volume != volume || j.Chapter != chapter || j.State != audio.JobError {
				continue
			}
			switch op {
			case "retry", "resume":
				return audioJobErr(entry.svc.Resume(j.ID))
			case "cancel":
				return audioJobErr(entry.svc.Cancel(j.ID))
			}
			return errJobUnsupported
		}
		return errJobNotFound
	}
	var acted bool
	for _, j := range jobs {
		if j.Volume != volume {
			continue
		}
		var err error
		switch op {
		case "pause":
			if j.State != audio.JobQueued && j.State != audio.JobRendering {
				continue
			}
			err = entry.svc.Pause(j.ID)
		case "resume":
			if j.State != audio.JobPaused {
				continue
			}
			err = entry.svc.Resume(j.ID)
		case "cancel":
			if !audioJobActive(j.State) {
				continue
			}
			err = entry.svc.Cancel(j.ID)
		default:
			return errJobUnsupported
		}
		if err != nil {
			return audioJobErr(err)
		}
		acted = true
	}
	if !acted {
		return errJobNotFound
	}
	return nil
}

func audioJobErr(err error) error {
	if err == audio.ErrNotFound {
		return errJobNotFound
	}
	return err
}

// parseAudioJobID 解析 "<p>/<s>/audio/<N>[/<ch>]"。
func parseAudioJobID(id string) (audioEntry, int, int, bool) {
	parts := strings.Split(id, "/")
	if len(parts) < 4 || len(parts) > 5 || parts[2] != "audio" {
		return audioEntry{}, 0, 0, false
	}
	volume, err := strconv.Atoi(parts[3])
	if err != nil || volume <= 0 {
		return audioEntry{}, 0, 0, false
	}
	chapter := 0
	if len(parts) == 5 {
		if chapter, err = strconv.Atoi(parts[4]); err != nil || chapter <= 0 {
			return audioEntry{}, 0, 0, false
		}
	}
	var found audioEntry
	audioServices.Range(func(_, v any) bool {
		e := v.(audioEntry)
		if e.project == parts[0] && e.story == parts[1] {
			found = e
			return false
		}
		return true
	})
	return found, volume, chapter, found.svc != nil
}

func audioJobActive(state string) bool {
	return state == audio.JobQueued || state == audio.JobRendering || state == audio.JobPaused
}

// audioRows 把一个故事的队列折成任务中心的行。machineReady=false 时，
// 还在 xếp hàng 的生成行带上 waitingMachine=true（§AT §8.3：等的是 máy đọc）。
func audioRows(entry audioEntry, now time.Time, machineReady bool) []Job {
	jobs := entry.svc.Queue()
	spa := "/p/" + url.PathEscape(entry.project) + "/s/" + url.PathEscape(entry.story) + "/audio/"
	idBase := entry.project + "/" + entry.story + "/audio/"
	byVolume := map[int][]audio.Job{}
	anyRendering := false
	for _, j := range jobs {
		byVolume[j.Volume] = append(byVolume[j.Volume], j)
		anyRendering = anyRendering || j.State == audio.JobRendering
	}
	volumes := make([]int, 0, len(byVolume))
	for v := range byVolume {
		volumes = append(volumes, v)
	}
	sort.Ints(volumes)

	var rows []Job
	for _, request := range entry.svc.ExportRequests() {
		vol := strconv.Itoa(request.Volume)
		row := exportJobRow(entry, request, spa, idBase, vol)
		if row.ID == "" {
			continue
		}
		rows = append(rows, row)
	}

	for _, volume := range volumes {
		list := byVolume[volume]
		vol := strconv.Itoa(volume)
		base := Job{Kind: "audio", Project: entry.project, Story: entry.story}

		// 出错的章节各占一行。
		for _, j := range list {
			if j.State != audio.JobError {
				continue
			}
			row := base
			row.ID = idBase + vol + "/" + strconv.Itoa(j.Chapter)
			// §AK：错误行按任务类型起名，与生成行的 "Đang tạo / Đang cập nhật" 对齐。
			switch j.Kind {
			case audio.JobKindUpdate:
				row.Title = fmt.Sprintf("Cập nhật giọng chương %d", j.Chapter)
			case audio.JobKindRedo:
				row.Title = fmt.Sprintf("Tạo lại giọng chương %d", j.Chapter)
			default:
				row.Title = fmt.Sprintf("Tạo giọng chương %d", j.Chapter)
			}
			row.Status = "failed"
			row.Error = j.Error
			normalizeJobError(&row)
			row.Href = spa + vol + "/make"
			row.StartedAt = j.CreatedAt
			rows = append(rows, row)
		}

		// 本轮 = 仍在跑的任务 + 本轮开始后完成的任务；没有在跑的就看最近一批完成的。
		var runStart, finished time.Time
		active := 0
		for _, j := range list {
			if audioJobActive(j.State) {
				active++
				if runStart.IsZero() || j.CreatedAt.Before(runStart) {
					runStart = j.CreatedAt
				}
			}
			if j.State == audio.JobDone && j.UpdatedAt.After(finished) {
				finished = j.UpdatedAt
			}
		}
		if active == 0 {
			if finished.IsZero() {
				continue
			}
			runStart = finished.Add(-time.Hour)
		}
		var run []audio.Job
		for _, j := range list {
			if audioJobActive(j.State) || (j.State == audio.JobDone && !j.UpdatedAt.Before(runStart)) {
				run = append(run, j)
			}
		}
		row := base
		row.ID = idBase + vol
		row.Title = "Tạo giọng Tập " + vol
		row.Href = spa + vol + "/make"
		row.StartedAt = runStart
		row.Unit = audioRunUnit(run)
		row.Progress = audioRunProgress(run)
		paused, rendering := 0, false
		for _, j := range run {
			switch j.State {
			case audio.JobPaused:
				paused++
			case audio.JobRendering:
				rendering = true
				row.ETASec = int(j.ETASec)
				row.Step = fmt.Sprintf("Chương %d", j.Chapter)
				if j.Step == "compose" {
					row.Step += " · Đang soạn bản đọc"
				} else if j.Speaker != "" {
					row.Step += " · " + j.Speaker
				}
				if j.Kind == audio.JobKindUpdate && j.Step != "compose" {
					row.Step += " · Đang cập nhật"
				}
			}
		}
		switch {
		case active == 0:
			row.Status = "done"
			at := finished
			row.FinishedAt = &at
			row.Title = "Đã tạo giọng Tập " + vol
			row.Result = &JobResult{
				Title: "Đã tạo giọng Tập " + vol,
				Sub:   fmt.Sprintf("%d chương", len(run)),
				Actions: []JobAction{
					{Label: "Nghe", Primary: true, Kind: "open", Href: spa + vol + "/listen"},
					{Label: "Ghép & xuất", Kind: "open", Href: spa + vol},
				},
			}
		case paused == active:
			row.Status = "paused"
		case rendering:
			row.Status = "running"
		case anyRendering:
			row.Status = "waiting" // 引擎串行：别的卷正在用
		default:
			row.Status = "running"
		}
		// Việc còn xếp hàng nhưng không có máy đọc sẵn sàng: báo thẳng
		// "đang chờ máy đọc" thay vì giả vờ đang đọc (§AT §8.3).
		if !machineReady && audioHasQueued(run) {
			row.WaitingMachine = true
		}
		rows = append(rows, row)
	}
	return rows
}

// audioHasQueued: trong lượt có chương đang chờ claim (queued, không paused).
func audioHasQueued(run []audio.Job) bool {
	for _, j := range run {
		if j.State == audio.JobQueued {
			return true
		}
	}
	return false
}

// exportJobRow 折出一条 §AE 的导出任务行：kind=export、title="Ghép Tập N"、
// unit 按章数、error 是 {message,chapter,retryable,cause}、result 带 files/zipUrl。
func exportJobRow(entry audioEntry, request audio.ExportRequest, spa, idBase, vol string) Job {
	title := "Ghép Tập " + vol
	if request.Options.Trial && len(request.Options.Chapters) == 1 {
		title = fmt.Sprintf("Xuất thử chương %d", request.Options.Chapters[0])
	} else if request.Options.Split {
		title = fmt.Sprintf("Xuất %d chương", len(request.Options.Chapters))
	}
	total := len(request.Source)
	if total == 0 && len(request.Options.Chapters) > 0 {
		total = len(request.Options.Chapters)
	}
	row := Job{
		ID:        idBase + vol + "/export",
		Kind:      "export",
		Project:   entry.project,
		Story:     entry.story,
		Title:     title,
		Href:      spa + vol,
		StartedAt: request.CreatedAt,
	}
	percent := request.Percent / 100
	row.Progress = &percent
	row.Step = request.Step
	row.ETASec = request.EtaSec
	if total > 0 {
		row.Unit = &JobUnit{Done: request.ChaptersDone, Total: total, Label: "Chương"}
	}
	switch request.State {
	case audio.JobQueued:
		row.Status = JobWaiting
		row.Step = "Đang chờ lượt ghép"
		if request.WaitReady {
			row.Step = "Chờ tạo giọng xong"
		}
	case audio.JobRendering:
		row.Status = JobRunning
		if row.Step == "" {
			row.Step = "Đang ghép và xuất"
		}
	case audio.JobError:
		row.Status = JobFailed
		info := JobErrorInfo{Message: request.Error, Retryable: request.Retryable, Cause: request.ErrorCause}
		if request.ErrorChapter > 0 {
			ch := request.ErrorChapter
			info.Chapter = &ch
		}
		row.Error = info
	case audio.JobDone:
		p := 1.0
		row.Progress = &p
		row.Status = JobDoneS
		at := request.UpdatedAt
		row.FinishedAt = &at
		row.Title = "Đã ghép Tập " + vol
		if request.Options.Trial {
			row.Title = "Bản thử đã sẵn sàng"
		}
		row.Result = exportJobResult(row.Title, request, spa+vol)
	default:
		return Job{}
	}
	return row
}

// exportJobResult 组 §AE 的结果卡：files[]（卷成品在前）+ zipUrl + 下载/打开按钮。
func exportJobResult(title string, request audio.ExportRequest, openHref string) *JobResult {
	res := &JobResult{Title: title}
	if request.Result == nil {
		res.Sub = fmt.Sprintf("%.0f giây · %s", request.DurationSec, strings.ToUpper(request.Options.Format))
		res.Actions = []JobAction{{Label: "Nghe & tải", Kind: "open", Href: openHref, Primary: true}}
		return res
	}
	files := append([]audio.ExportFile(nil), request.Result.Files...)
	// files[0] = 卷成品（FE 自下载它）：非 chuong/ 目录的排在前面。
	sort.SliceStable(files, func(i, j int) bool {
		return !strings.Contains(files[i].Path, "/chuong") && strings.Contains(files[j].Path, "/chuong")
	})
	res.Files = make([]JobResultFile, 0, len(files))
	var totalBytes int64
	for _, f := range files {
		fileURL := f.URL
		if fileURL == "" {
			fileURL = "/exports/file?path=" + url.QueryEscape(f.Path)
		}
		res.Files = append(res.Files, JobResultFile{
			Name: f.Name, SizeBytes: f.SizeBytes, URL: fileURL, Path: f.Path,
			Chapter: f.Chapter, DurationSec: f.DurationSec,
		})
		totalBytes += f.SizeBytes
	}
	res.ZipURL = request.Result.ZipURL
	res.Sub = fmt.Sprintf("%.0f giây · %s", request.Result.DurationSec, strings.ToUpper(request.Options.Format))
	if totalBytes > 0 {
		res.Sub += " · " + humanBytes(int(totalBytes))
	}
	download := res.ZipURL
	if download == "" && len(res.Files) > 0 {
		download = res.Files[0].URL
	}
	res.Actions = []JobAction{}
	if download != "" {
		res.Actions = append(res.Actions, JobAction{Label: "Tải xuống", Kind: "download", Href: download, Primary: true})
	}
	res.Actions = append(res.Actions, JobAction{Label: "Xem bản xuất", Kind: "open", Href: openHref})
	res.Data = map[string]any{"files": res.Files, "zipUrl": res.ZipURL}
	return res
}

func audioRunUnit(run []audio.Job) *JobUnit {
	done := 0
	for _, j := range run {
		if j.State == audio.JobDone {
			done++
		}
	}
	return &JobUnit{Done: done, Total: len(run), Label: "Chương"}
}

// audioRunProgress 按段数加权；还没开始的章节段数未知，按已知章节的平均段数估。
func audioRunProgress(run []audio.Job) *float64 {
	var done, total float64
	known, knownSegs := 0, 0
	for _, j := range run {
		if j.Segments > 0 {
			known++
			knownSegs += j.Segments
		}
	}
	avg := 1.0
	if known > 0 {
		avg = float64(knownSegs) / float64(known)
	}
	for _, j := range run {
		segs := float64(j.Segments)
		if segs == 0 {
			segs = avg
		}
		total += segs
		switch {
		case j.State == audio.JobDone:
			done += segs
		default:
			done += float64(j.SegmentsDone)
		}
	}
	if total == 0 {
		return nil
	}
	p := done / total
	return &p
}
