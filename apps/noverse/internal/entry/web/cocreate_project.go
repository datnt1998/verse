package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// G8：đồng sáng tác ở cấp dự án, trước khi truyện tồn tại.
// Worker chạy với cwd = <dự án>/.ainovel/drafts/<id>/; commit chuyển thư mục nháp
// thành <dirs.stories>/NN-slug rồi khởi động worker của truyện và gửi bản nháp
// qua đúng đường chốt đồng sáng tác (POST /cocreate/commit).

// draftKey 给草稿 worker 一个不可能与故事 id 冲突的槽位键（故事 id 只有一个 "/"）。
func draftKey(project, id string) string { return "draft//" + project + "/" + id }

// newDraftID 生成草稿 id：时间戳 + 6 ký tự hex，可排序且是合法目录名。
func newDraftID() string {
	buf := make([]byte, 3)
	_, _ = rand.Read(buf)
	return time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(buf)
}

// serveProjectCoCreate 处理 /api/projects/:p/cocreate[/message|/commit|/cancel]。
func (g *Gateway) serveProjectCoCreate(w http.ResponseWriter, r *http.Request, lib library.Repository, user webauth.User, p string, segs []string) {
	if !g.requireProject(w, lib, r.Context(), actorOf(user), user, p) {
		return
	}
	if r.Method != http.MethodGet && !user.CanCollaborateProject(p) {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return
	}
	if p == library.LegacyProjectID {
		jsonError(w, http.StatusConflict, "Dự án ẩn danh không hỗ trợ đồng sáng tác")
		return
	}
	action := ""
	if len(segs) > 0 {
		action = segs[0]
	}
	switch {
	case action == "" && r.Method == http.MethodGet:
		g.getProjectCoCreate(w, r, lib, user, p)
	case action == "message" && r.Method == http.MethodPost:
		g.postProjectCoCreateMessage(w, r, lib, user, p)
	case action == "commit" && r.Method == http.MethodPost:
		g.postProjectCoCreateCommit(w, r, lib, user, p)
	case action == "cancel" && r.Method == http.MethodPost:
		g.postProjectCoCreateCancel(w, r, lib, user, p)
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
}

// draftIDFrom 取草稿 id：query ?draft= 优先，其次请求体字段。
func draftIDFrom(r *http.Request, body string) string {
	if q := strings.TrimSpace(r.URL.Query().Get("draft")); q != "" {
		return q
	}
	return strings.TrimSpace(body)
}

// draftDirOf 校验草稿 id 并返回目录；不存在时返回 404。
func (g *Gateway) draftDirOf(w http.ResponseWriter, ctx context.Context, lib library.Repository, actor *library.Actor, p, id string) (string, bool) {
	if !library.ValidDraftID(id) {
		jsonError(w, http.StatusBadRequest, "draft không hợp lệ")
		return "", false
	}
	dir, err := lib.DraftDir(ctx, actor, p, id)
	if err != nil {
		libError(w, err)
		return "", false
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		jsonError(w, http.StatusNotFound, "không tìm thấy bản nháp")
		return "", false
	}
	return dir, true
}

// getProjectCoCreate 返回草稿的共创状态；没有 draft 参数时返回空状态。
func (g *Gateway) getProjectCoCreate(w http.ResponseWriter, r *http.Request, lib library.Repository, user webauth.User, p string) {
	id := draftIDFrom(r, "")
	if id == "" {
		writeJSON(w, http.StatusOK, emptyCoCreateState(""))
		return
	}
	dir, ok := g.draftDirOf(w, r.Context(), lib, actorOf(user), p, id)
	if !ok {
		return
	}
	worker, err := g.startDir(draftKey(p, id), dir)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	status, body, err := g.workerCall(r.Context(), worker, http.MethodGet, "cocreate", nil)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if status != http.StatusOK {
		jsonError(w, status, workerError(status, body))
		return
	}
	var state map[string]any
	if err := json.Unmarshal(body, &state); err != nil {
		jsonError(w, http.StatusBadGateway, "Phản hồi đồng sáng tác từ worker không đọc được: "+jsonBodyMessage(err))
		return
	}
	state["draftId"] = id
	writeJSON(w, http.StatusOK, state)
}

// emptyCoCreateState 与 worker 的 GET /cocreate 同形，另加 draftId。
func emptyCoCreateState(id string) map[string]any {
	return map[string]any{
		"schemaVersion": 1, "proposal": nil, "committed": false, "nextChapter": nil,
		"draftId": id, "active": false, "messages": []any{},
		"draft": "", "ready": false, "suggestions": []string{},
	}
}

// postProjectCoCreateMessage 追加用户消息并同步跑一轮；首条消息没有 draft 时新建草稿。
func (g *Gateway) postProjectCoCreateMessage(w http.ResponseWriter, r *http.Request, lib library.Repository, user webauth.User, p string) {
	var req struct {
		Message string `json:"message"`
		Draft   string `json:"draft"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	text := strings.TrimSpace(req.Message)
	if text == "" {
		jsonError(w, http.StatusBadRequest, "message không được rỗng")
		return
	}
	id := draftIDFrom(r, req.Draft)
	var dir string
	if id == "" {
		id = newDraftID()
		created, err := lib.CreateDraft(r.Context(), actorOf(user), p, id)
		if err != nil {
			libError(w, err)
			return
		}
		dir = created
	} else {
		var ok bool
		if dir, ok = g.draftDirOf(w, r.Context(), lib, actorOf(user), p, id); !ok {
			return
		}
	}
	worker, err := g.startDir(draftKey(p, id), dir)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	status, body, err := g.workerCall(r.Context(), worker, http.MethodPost, "cocreate/message", map[string]string{"message": text})
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if status != http.StatusOK {
		jsonError(w, status, workerError(status, body))
		return
	}
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		jsonError(w, http.StatusBadGateway, "Phản hồi đồng sáng tác từ worker không đọc được: "+jsonBodyMessage(err))
		return
	}
	resp["draftId"] = id
	writeJSON(w, http.StatusOK, resp)
}

// postProjectCoCreateCommit 把草稿变成正式 truyện rồi bắt đầu viết.
func (g *Gateway) postProjectCoCreateCommit(w http.ResponseWriter, r *http.Request, lib library.Repository, user webauth.User, p string) {
	var req struct {
		Name  string `json:"name"`
		Draft string `json:"draft"`
		Brief string `json:"brief"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	id := draftIDFrom(r, req.Draft)
	if id == "" {
		jsonError(w, http.StatusBadRequest, "thiếu tham số draft")
		return
	}
	dir, ok := g.draftDirOf(w, r.Context(), lib, actorOf(user), p, id)
	if !ok {
		return
	}
	worker, err := g.startDir(draftKey(p, id), dir)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	status, body, err := g.workerCall(r.Context(), worker, http.MethodGet, "cocreate", nil)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if status != http.StatusOK {
		jsonError(w, status, workerError(status, body))
		return
	}
	var state struct {
		Active   bool                   `json:"active"`
		Messages []json.RawMessage      `json:"messages"`
		Draft    string                 `json:"draft"`
		Proposal *host.CoCreateProposal `json:"proposal"`
	}
	if err := json.Unmarshal(body, &state); err != nil {
		jsonError(w, http.StatusBadGateway, "Phản hồi đồng sáng tác từ worker không đọc được: "+jsonBodyMessage(err))
		return
	}
	if !state.Active {
		jsonError(w, http.StatusConflict, "Chưa có phiên đồng sáng tác nào đang mở")
		return
	}
	draft := strings.TrimSpace(state.Draft)
	if draft == "" {
		jsonError(w, http.StatusBadRequest, "Chưa có bản nháp chỉ dẫn sáng tác, gửi thêm tin nhắn để AI tổng hợp")
		return
	}
	if strings.TrimSpace(req.Brief) != "" {
		draft = strings.TrimSpace(req.Brief)
	}
	if !g.ledgerStartGate(w, r, lib, user, p, draft, "") {
		return
	}
	// Tên truyện chỉ lấy từ name người dùng gõ — nội dung bản nháp/tin nhắn là yêu cầu,
	// không phải tên. Trống → thư mục tạm NN-truyen-moi (tempName), tiêu đề trống chờ
	// engine đặt rồi đồng bộ vào story.json.
	name := strings.TrimSpace(req.Name)
	// Dừng worker nháp trước khi đổi tên thư mục (nó đang giữ file trong đó).
	g.dropSlot(draftKey(p, id))
	story, _, err := lib.CommitDraft(r.Context(), actorOf(user), p, id, name)
	if err != nil {
		libError(w, err)
		return
	}
	result := map[string]any{
		"schemaVersion": 1, "story": g.storySummary(lib, story, &user),
		"proposal": state.Proposal, "committed": false, "nextChapter": coCreateNextChapter(state.Proposal),
	}
	// Worker mới khôi phục phiên đã chuyển thư mục rồi chốt bằng đường đồng sáng tác
	// thật (PrepareUserRules/StartPrepared), giữ đề xuất và trạng thái đã chốt.
	storyWorker, err := g.startDir(story.ID, story.Dir)
	if err != nil {
		result["warning"] = "Đã tạo truyện nhưng chưa bắt đầu viết: " + err.Error()
		writeJSON(w, http.StatusCreated, result)
		return
	}
	startCtx, cancelStart := context.WithTimeout(context.Background(), 5*time.Minute)
	// Lượt nền tự đóng context sau khi kết thúc, kể cả khi UI đã nhận 201.
	type startResult struct {
		status int
		body   []byte
		err    error
	}
	started := make(chan startResult, 1)
	go func() {
		defer cancelStart()
		status, body, err := g.workerCall(startCtx, storyWorker, http.MethodPost, "cocreate/commit", map[string]string{"draft": draft})
		started <- startResult{status: status, body: body, err: err}
	}()
	var res startResult
	select {
	case res = <-started:
	case <-time.After(3 * time.Second):
		// Engine vẫn đang khởi động; truyện đã có, worker tiếp tục ở nền.
		writeJSON(w, http.StatusCreated, result)
		return
	}
	if res.err == nil && res.status == http.StatusAccepted {
		// The worker accepted the start; Arbiter/model progress continues as events.
		writeJSON(w, http.StatusCreated, result)
		return
	}
	if res.err != nil || res.status != http.StatusOK {
		msg := ""
		if res.err != nil {
			msg = res.err.Error()
		} else {
			msg = workerError(res.status, res.body)
		}
		result["warning"] = "Đã tạo truyện nhưng chưa bắt đầu viết: " + msg
		writeJSON(w, http.StatusCreated, result)
		return
	}
	if f, err := lib.LoadStoryFile(r.Context(), actorOf(user), story); err == nil && f != nil {
		f.Status = "writing"
		if err := lib.WriteStoryFile(r.Context(), actorOf(user), story, *f); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	result["story"] = g.storySummary(lib, story, &user)
	result["committed"] = true
	writeJSON(w, http.StatusCreated, result)
}

// postProjectCoCreateCancel 取消 phiên và xoá thư mục nháp.
func (g *Gateway) postProjectCoCreateCancel(w http.ResponseWriter, r *http.Request, lib library.Repository, user webauth.User, p string) {
	var req struct {
		Draft string `json:"draft"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	id := draftIDFrom(r, req.Draft)
	if id == "" {
		jsonError(w, http.StatusBadRequest, "thiếu tham số draft")
		return
	}
	if _, ok := g.draftDirOf(w, r.Context(), lib, actorOf(user), p, id); !ok {
		return
	}
	if b := g.slot(draftKey(p, id)); b != nil {
		b.mu.Lock()
		worker := b.worker
		b.mu.Unlock()
		if worker != nil && !workerDead(worker) {
			// 让 worker 清会话（不阻塞太久：worker 可能正在跑一轮 LLM）。
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			_, _, _ = g.workerCall(ctx, worker, http.MethodPost, "cocreate/cancel", struct{}{})
			cancel()
		}
	}
	g.dropSlot(draftKey(p, id))
	if err := lib.RemoveDraft(r.Context(), actorOf(user), p, id); err != nil {
		libError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
