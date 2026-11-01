package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/platform/identity"
)

// commentStoreFor binds the comment lifecycle for a story: file mode uses the
// story directory, hosted mode uses the scoped comment rows opened with the
// request actor. A hosted backend without a comment surface is a capability
// error, never a directory fallback.
func (g *Gateway) commentStoreFor(user identity.User, st library.Story) (*comments.Store, error) {
	if g.factStore == nil {
		return comments.New(st.Dir), nil
	}
	engine, err := g.openFactStore(st, &user)
	if err != nil {
		return nil, err
	}
	cs := engine.Comments()
	if cs == nil {
		return nil, fmt.Errorf("kho dữ kiện không cung cấp góp ý: %w", errs.ErrStorageUnsupported)
	}
	return cs, nil
}

// serveComments runs at the gateway, without acquiring the engine story lease.
func (g *Gateway) serveComments(w http.ResponseWriter, r *http.Request, user identity.User) bool {
	seg := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(seg) < 6 || seg[0] != "api" || seg[1] != "projects" || seg[3] != "stories" {
		return false
	}
	isComments := len(seg) == 8 && seg[5] == "chapters" && seg[7] == "comments"
	isCommentItem := len(seg) == 9 && seg[5] == "chapters" && seg[7] == "comments"
	isNext := len(seg) == 6 && seg[5] == "next" && r.Method == http.MethodPost
	if !isComments && !isCommentItem && !isNext {
		return false
	}
	id := seg[2] + "/" + seg[4]
	st, err := g.resolveStoryID(id)
	if err != nil {
		jsonError(w, 404, "Không tìm thấy truyện")
		return true
	}
	// 与 serveStories 同一道门：GET 需可读、其余需可写；未授权一律 404。
	if !storyAccessAllowed(user, st, r.Method, "") {
		jsonError(w, 404, "Không tìm thấy truyện")
		return true
	}
	changed := func() {
		g.events().Publish(Event{Type: "story.changed", Project: st.Project, Story: st.Story, Data: map[string]any{"parts": []string{"comments"}}})
	}
	if isNext {
		// D4: cửa duyệt chỉ mở khi bài thẩm duyệt mới nhất của chương không còn
		// mục Bắt sửa. Chặn ở gateway để cả POST …/next lẫn thao tác "approve"
		// của thông báo (notify.go gọi lại chính hàm này) nhận cùng một 409.
		var body []byte
		if r.Body != nil {
			body, _ = io.ReadAll(io.LimitReader(r.Body, 1<<20))
		}
		var gate struct {
			ApplyComments bool `json:"applyComments"`
			Chapter       int  `json:"chapter"`
		}
		if len(bytes.TrimSpace(body)) > 0 {
			_ = json.Unmarshal(body, &gate)
		}
		sr, srErr := g.openStoryReader(user, st)
		if srErr != nil {
			jsonError(w, http.StatusInternalServerError, srErr.Error())
			return true
		}
		if chapter, items := blockingReviewItems(st, sr, gate.Chapter); len(items) > 0 {
			writeJSON(w, 409, map[string]any{
				"error": fmt.Sprintf(reviewApproveBlockedBy, chapter),
				"items": items,
			})
			return true
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		// Thân đã đọc và dựng lại: Content-Length của request gốc có thể lệch
		// (notify.go dựng thân mới cho thao tác "approve"), để nguyên thì
		// ReverseProxy trả 502 "ContentLength=… with Body length …".
		r.ContentLength = int64(len(body))
		rec := &commentStatusWriter{ResponseWriter: w, status: 200}
		g.proxyToWorker(rec, r, id, "next")
		if rec.status < 300 {
			changed()
		}
		return true
	}
	chapter, err := strconv.Atoi(seg[6])
	if err != nil || chapter < 1 {
		jsonError(w, 400, "Số chương không hợp lệ")
		return true
	}
	store, storeErr := g.commentStoreFor(user, st)
	if storeErr != nil {
		commentError(w, storeErr)
		return true
	}
	if isCommentItem {
		commentID := seg[8]
		switch r.Method {
		case http.MethodPatch:
			if !commentChangeAllowed(w, store, chapter, commentID, user, st) {
				return true
			}
			var in comments.PatchInput
			if !decodeJSON(w, r, &in) {
				return true
			}
			comment, err := store.Patch(chapter, commentID, in)
			if err != nil {
				commentError(w, err)
				return true
			}
			changed()
			writeJSON(w, 200, comment)
		case http.MethodDelete:
			if !commentChangeAllowed(w, store, chapter, commentID, user, st) {
				return true
			}
			if err := store.Delete(chapter, commentID); err != nil {
				commentError(w, err)
				return true
			}
			changed()
			writeJSON(w, 200, map[string]any{"ok": true})
		default:
			jsonError(w, 405, "Phương thức không được hỗ trợ")
		}
		return true
	}
	switch r.Method {
	case http.MethodGet:
		list, err := store.List(chapter)
		if err != nil {
			commentError(w, err)
			return true
		}
		writeJSON(w, 200, map[string]any{"comments": list})
	case http.MethodPost:
		var in comments.Input
		if !decodeJSON(w, r, &in) {
			return true
		}
		comment, err := store.Add(chapter, in, user.Username)
		if err != nil {
			commentError(w, err)
			return true
		}
		changed()
		writeJSON(w, 201, comment)
	case http.MethodDelete:
		commentID := r.URL.Query().Get("id")
		if commentID == "" && r.Body != nil {
			var in struct {
				ID string `json:"id"`
			}
			if !decodeJSON(w, r, &in) {
				return true
			}
			commentID = in.ID
		}
		if commentID == "" {
			jsonError(w, 400, "Thiếu mã góp ý")
			return true
		}
		if !commentChangeAllowed(w, store, chapter, commentID, user, st) {
			return true
		}
		if err := store.Delete(chapter, commentID); err != nil {
			commentError(w, err)
			return true
		}
		changed()
		writeJSON(w, 200, map[string]any{"ok": true})
	default:
		jsonError(w, 405, "Phương thức không được hỗ trợ")
	}
	return true
}

type commentStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *commentStatusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func commentError(w http.ResponseWriter, err error) {
	// Invalid/not-found comment sentinels and the shared storage family are all
	// classified by the one policy: a storage conflict here is a recoverable
	// 409, not a 500 with the cause echoed back.
	jsonMappedError(w, err)
}

// commentChangeForbidden 是 sửa/xoá góp ý của người khác mà không phải chủ dự án
// hay quản trị viên. Câu tiếng Việt hiện thẳng trong toast của màn Đọc.
const commentChangeForbidden = "Chỉ người tạo góp ý, chủ dự án hoặc quản trị viên mới sửa hoặc xoá được góp ý này"

// commentChangeAllowed 校验 PATCH/DELETE một góp ý: chỉ người tạo, chủ dự án
// hoặc quản trị viên. Ghi lỗi và trả false khi không được phép; góp ý không có
// vẫn là 404 như cũ (không lộ danh sách cho người không sửa được).
func commentChangeAllowed(w http.ResponseWriter, store *comments.Store, chapter int, id string, user identity.User, st library.Story) bool {
	author, err := store.Author(chapter, id)
	if err != nil {
		commentError(w, err)
		return false
	}
	if isAdmin(user) || user.CanManageProject(st.Project) || author == user.Username {
		return true
	}
	jsonError(w, http.StatusForbidden, commentChangeForbidden)
	return false
}

// commentStore is the worker-side comment lifecycle: the Host's backend store
// (file rows under the story dir, scoped rows on hosted). The file fallback is
// only for a Host that reports no surface, never a cwd-derived directory.
func (s *workerServer) commentStore() *comments.Store {
	if cs := s.h.CommentStore(); cs != nil {
		return cs
	}
	return comments.New(s.storyDir)
}

func (s *workerServer) postNext(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ApplyComments bool `json:"applyComments"`
		Chapter       int  `json:"chapter"`
	}
	if r.ContentLength != 0 && !decodeJSON(w, r, &in) {
		return
	}
	var (
		advanceErr    error
		applied       int
		applyComments = in.ApplyComments
	)
	if applyComments && in.Chapter == 0 {
		chapters, err := s.h.Chapters()
		if err != nil {
			jsonError(w, 409, err.Error())
			return
		}
		for _, ch := range chapters {
			if ch.Completed && ch.Chapter > in.Chapter {
				in.Chapter = ch.Chapter
			}
		}
		if in.Chapter < 1 {
			applyComments = false
		}
	}
	if applyComments {
		applied, advanceErr = s.commentStore().Apply(in.Chapter, func(instruction string) error {
			return s.h.AdvanceOneChapterWithComments(in.Chapter, instruction)
		})
	} else {
		advanceErr = s.h.AdvanceOneChapter()
	}
	if advanceErr == nil {
		// §AI: lần duyệt kế tiếp của truyện chuyển mọi góp ý Đã sửa → Đã xử lý.
		// Quét sau khi duyệt thành công; lỗi quét không đánh hỏng lần duyệt đã nhận.
		if err := s.commentStore().ResolveAllFixed(); err != nil {
			slog.Warn("chuyển góp ý đã sửa sang đã xử lý thất bại", "module", "web", "chapter", in.Chapter, "err", err)
		}
	}
	commandResult(w, advanceErr, map[string]any{"ok": true, "appliedComments": applied})
}
