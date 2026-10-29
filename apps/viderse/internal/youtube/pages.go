package youtube

import (
	"context"
	"strings"
)

// ---------------------------------------------------------------------------
// Điểm vào quét kênh
// ---------------------------------------------------------------------------

// guarded bọc một lời gọi provider: lỗi ctx và DeferredError đi nguyên vẹn,
// mọi thứ khác chuẩn hoá thành FetchError theo ngữ cảnh trang đầu/tiếp.
func (c *Client) guarded(ctx context.Context, continuation bool, work func() (*ChannelVideoPage, error)) (*ChannelVideoPage, error) {
	page, err := work()
	if err != nil {
		return nil, normalizeError(err, continuation, c.now())
	}
	return page, nil
}

// ResolveChannelRef là ResolveChannelID với lỗi provider đã phân loại
// (UC id hợp lệ không tốn request nào).
func (c *Client) ResolveChannelRef(ctx context.Context, channel string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	id, err := c.ResolveChannelID(ctx, channel)
	if err != nil {
		return "", normalizeError(err, false, c.now())
	}
	return id, nil
}

// FirstChannelPage đọc trang đầu của tab Videos theo thứ tự Mới nhất cùng hồ sơ
// kênh. channel là @handle, URL kênh hoặc channel id; UC id không tốn request phân giải.
func (c *Client) FirstChannelPage(ctx context.Context, channel string) (*ChannelVideoPage, error) {
	return c.guarded(ctx, false, func() (*ChannelVideoPage, error) {
		id, err := c.ResolveChannelID(ctx, channel)
		if err != nil {
			return nil, err
		}
		sess, err := c.session(ctx, "")
		if err != nil {
			return nil, err
		}
		home, err := sess.browseChannel(ctx, id)
		if err != nil {
			return nil, err
		}
		profile := c.profileOf(id, home)
		// Kênh chỉ có Shorts/Live không có tab Videos; không thay tab khác vào.
		videosTab := findTabByURLSuffix(home, "videos")
		if videosTab == nil {
			return &ChannelVideoPage{Videos: []ScannedVideo{}, Profile: profile}, nil
		}
		tabPage := home
		if !isTabSelected(videosTab) {
			tabPage, err = sess.callEndpoint(ctx, jobj(videosTab["endpoint"]))
			if err != nil {
				return nil, err
			}
		}
		feed, err := c.latestVideosFeed(ctx, sess, tabPage)
		if err != nil {
			return nil, err
		}
		videos, next, err := c.readPage(feed, id, sess)
		if err != nil {
			return nil, err
		}
		return &ChannelVideoPage{Videos: videos, NextCursor: next, Profile: profile}, nil
	})
}

// NextChannelPage đọc trang sau cursor; token continuation được phát lại nguyên
// vẹn trên session cùng visitor data đã phát hành nó.
func (c *Client) NextChannelPage(ctx context.Context, cursor ChannelCursor) (*ChannelVideoPage, error) {
	return c.guarded(ctx, true, func() (*ChannelVideoPage, error) {
		if err := validateCursor(&cursor); err != nil {
			return nil, invalidCursorErr("Cursor is invalid or incompatible")
		}
		sess, err := c.sessionForCursor(ctx, cursor)
		if err != nil {
			return nil, err
		}
		resp, err := sess.execute(ctx, "/browse", jmap{"continuation": cursor.Continuation.Token})
		if err != nil {
			return nil, err
		}
		// Token bị từ chối có thể về 200 mà không có action danh sách — đó
		// không phải tab rỗng.
		action := firstListAction(resp)
		if action == nil {
			return nil, invalidCursorErr("Continuation response carried no list items")
		}
		videos, next, err := c.readPage(resp, cursor.ChannelID, sess)
		if err != nil {
			return nil, err
		}
		return &ChannelVideoPage{Videos: videos, NextCursor: next}, nil
	})
}

// sessionForCursor trả session đã phát hành cursor (cùng visitor data) và kiểm
// session đó còn nói đúng client version của cursor.
func (c *Client) sessionForCursor(ctx context.Context, cursor ChannelCursor) (*session, error) {
	wanted := cursor.Session
	sess, err := c.session(ctx, wanted.VisitorData)
	if err != nil {
		return nil, err
	}
	actual := sess.context()
	if actual.ClientName != wanted.ClientName || actual.ClientVersion != wanted.ClientVersion {
		return nil, invalidCursorErr("Cursor was issued for a different YouTube client version")
	}
	return sess, nil
}

// ---------------------------------------------------------------------------
// Đọc trang / tab
// ---------------------------------------------------------------------------

// browseChannel gọi /browse cho channel home; nếu phản hồi là một navigateAction
// thì đi theo endpoint của nó (youtubei.js xử lý redirect này trong execute).
func (s *session) browseChannel(ctx context.Context, id string) (jmap, error) {
	resp, err := s.execute(ctx, "/browse", jmap{"browseId": id})
	if err != nil {
		return nil, err
	}
	for {
		actions := jarr(resp["onResponseReceivedActions"])
		nav := jobj(jget(firstOf(actions), "navigateAction"))
		if nav == nil {
			break
		}
		ep := jobj(nav["endpoint"])
		if ep == nil {
			break
		}
		resp, err = s.callEndpoint(ctx, ep)
		if err != nil {
			return nil, err
		}
	}
	if err := channelPageError(resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func firstOf(a []any) any {
	if len(a) == 0 {
		return nil
	}
	return a[0]
}

// channelPageError là lỗi của constructor Channel bên TS: alert ERROR = kênh
// không khả dụng; thiếu cả metadata lẫn contents = trang không dùng được.
func channelPageError(page jmap) error {
	for _, a := range jarr(page["alerts"]) {
		alert := jobj(jobj(a)["alertRenderer"])
		if alert == nil {
			continue
		}
		if jstr(alert["type"]) == "ERROR" {
			text := textOf(alert["text"])
			if text == "" {
				text = "Channel not available"
			}
			return newFetchError(KindNotFound, sanitizeDiagnostic(text), 0, nil)
		}
	}
	if page["metadata"] == nil && page["contents"] == nil {
		return &errInnertube{msg: "Invalid channel"}
	}
	return nil
}

// findTabByURLSuffix tìm tab có endpoint url kết thúc bằng /suffix.
func findTabByURLSuffix(page jmap, suffix string) jmap {
	for _, t := range jarr(jget(page, "contents", "twoColumnBrowseResultsRenderer", "tabs")) {
		tab := jobj(jobj(t)["tabRenderer"])
		if tab == nil {
			continue
		}
		u := jstr(jget(tab, "endpoint", "commandMetadata", "webCommandMetadata", "url"))
		if strings.HasSuffix(u, "/"+suffix) {
			return tab
		}
	}
	return nil
}

func isTabSelected(tab jmap) bool {
	sel, _ := tab["selected"].(bool)
	return sel
}

// callEndpoint gọi một NavigationEndpoint: apiUrl từ commandMetadata, body là
// payload (object con đầu tiên có khoá kết thúc "Endpoint"/"Command").
func (s *session) callEndpoint(ctx context.Context, endpoint jmap) (jmap, error) {
	inner := unwrapCommand(endpoint)
	if inner == nil {
		return nil, &errInnertube{msg: "endpoint has no command"}
	}
	apiPath := apiPathOf(inner)
	payload := endpointPayload(inner)
	if apiPath == "" {
		return nil, &errInnertube{msg: "endpoint has no api url"}
	}
	return s.execute(ctx, apiPath, payload)
}

// unwrapCommand mở các lớp bọc innertubeCommand/command/performOnceCommand/
// serviceEndpoint của một endpoint/command.
func unwrapCommand(v any) jmap {
	m := jobj(v)
	if m == nil {
		return nil
	}
	for _, k := range []string{"innertubeCommand", "command", "performOnceCommand", "serviceEndpoint"} {
		if inner := jobj(m[k]); inner != nil {
			return inner
		}
	}
	return m
}

// apiPathOf lấy apiUrl dạng "/browse" từ webCommandMetadata; thiếu apiUrl thì
// suy từ tên payload như youtubei.js (browseEndpoint → /browse).
func apiPathOf(endpoint jmap) string {
	if apiURL := jstr(jget(endpoint, "commandMetadata", "webCommandMetadata", "apiUrl")); apiURL != "" {
		return strings.TrimPrefix(apiURL, "/youtubei/v1")
	}
	for _, k := range jsortedKeys(endpoint) {
		if strings.HasSuffix(k, "Endpoint") || strings.HasSuffix(k, "Command") {
			switch k {
			case "browseEndpoint":
				return "/browse"
			case "watchEndpoint", "reelWatchEndpoint":
				return "/player"
			case "searchEndpoint":
				return "/search"
			}
		}
	}
	return ""
}

// endpointPayload trả object payload của endpoint: value của khoá đầu tiên kết
// thúc "Endpoint" hoặc "Command" (theo thứ tự khoá của map — TS dùng thứ tự khai
// báo key; ở đây dùng thứ tự bảng chữ cho ổn định, thực tế chỉ có một khoá).
func endpointPayload(endpoint jmap) jmap {
	for _, k := range jsortedKeys(endpoint) {
		if strings.HasSuffix(k, "Endpoint") || strings.HasSuffix(k, "Command") {
			if payload := jobj(endpoint[k]); payload != nil {
				return payload
			}
		}
	}
	return jmap{}
}

// firstListAction trả action đầu tiên trong onResponseReceivedActions/Endpoints
// nếu nó là appendContinuationItemsAction hoặc reloadContinuationItemsCommand.
func firstListAction(page jmap) jmap {
	for _, key := range []string{"onResponseReceivedActions", "onResponseReceivedEndpoints"} {
		actions := jarr(page[key])
		if len(actions) == 0 {
			continue
		}
		a := jobj(actions[0])
		if a == nil {
			return nil
		}
		if jobj(a["appendContinuationItemsAction"]) != nil {
			return a["appendContinuationItemsAction"].(map[string]any)
		}
		if jobj(a["reloadContinuationItemsCommand"]) != nil {
			return a["reloadContinuationItemsCommand"].(map[string]any)
		}
		return nil
	}
	return nil
}

// ---------------------------------------------------------------------------
// Đọc feed
// ---------------------------------------------------------------------------

// readPage rút videos + cursor kế từ một body feed (trang tab, trang lọc hay
// trang continuation). Renderer không đọc được chỉ mất một dòng; continuation
// không kiểm chứng được là lỗi, không phải hết tab.
func (c *Client) readPage(page jmap, channelID string, sess *session) ([]ScannedVideo, *ChannelCursor, error) {
	seen := map[string]bool{}
	videos := []ScannedVideo{}
	walkDFS(page, func(node jmap) bool {
		v := videoFromNode(node, c.now())
		if v != nil && !seen[v.ID] {
			seen[v.ID] = true
			videos = append(videos, *v)
		}
		return true
	})

	item := bodyContinuation(page)
	if item == nil {
		return videos, nil, nil
	}
	token, request := continuationToken(item)
	next := ChannelCursor{
		Version:      1,
		ChannelID:    channelID,
		Tab:          "videos",
		Order:        "latest",
		Continuation: Continuation{Token: token, Request: request},
		Session:      sess.context(),
	}
	if err := validateCursor(&next); err != nil {
		return nil, nil, newFetchError(KindUnexpectedPage,
			"YouTube sent a continuation this app cannot validate", 0, nil)
	}
	return videos, &next, nil
}

// bodyContinuation là item trang-kế của thân feed; continuation nằm trong
// header (vd tab "about") không tính.
func bodyContinuation(page jmap) jmap {
	// Subtree "header" của trang (vd tab "about") không phải danh sách: bỏ hẳn.
	for _, key := range jsortedKeys(page) {
		if key == "header" {
			continue
		}
		if found := firstContinuationNode(page[key]); found != nil {
			return found
		}
	}
	return nil
}

// firstContinuationNode tìm item trang-kế đầu tiên trong một nhánh JSON.
func firstContinuationNode(v any) jmap {
	var found jmap
	walkDFS(v, func(node jmap) bool {
		if item := jobj(node["continuationItemRenderer"]); item != nil {
			found = item
			return false
		}
		if item := jobj(node["continuationItemViewModel"]); item != nil {
			found = item
			return false
		}
		return true
	})
	return found
}

// continuationToken đọc (token, request) từ một continuation item renderer.
func continuationToken(item jmap) (string, string) {
	ep := unwrapCommand(item["continuationEndpoint"])
	if ep != nil {
		cmd := jobj(ep["continuationCommand"])
		if cmd == nil {
			// continuationItemViewModel: endpoint chính là continuationCommand
			cmd = ep
		}
		return jstr(cmd["token"]), jstr(cmd["request"])
	}
	if cmd := jobj(item["continuationCommand"]); cmd != nil {
		return jstr(cmd["token"]), jstr(cmd["request"])
	}
	return "", ""
}

// ---------------------------------------------------------------------------
// Chip lọc (Latest)
// ---------------------------------------------------------------------------

// filterChip là một chip sắp/lọc của feed.
type filterChip struct {
	label    string
	selected bool
	endpoint jmap
}

var dropDownTypes = map[string]bool{
	"CHIP_VIEW_MODEL_DISPLAY_TYPE_DROP_DOWN":            true,
	"CHIP_VIEW_MODEL_DISPLAY_TYPE_DROP_DOWN_WITH_CLEAR": true,
}

// filterChips trả chip của feed: [] rỗng khi feed không có thanh lọc, nil khi
// có thanh nhưng không đọc được (không xác nhận được thứ tự sắp xếp).
func filterChips(page jmap) []filterChip {
	if len(collectNodes(page, "feedFilterChipBarRenderer")) > 0 {
		var chips []filterChip
		for _, chip := range collectNodes(page, "chipCloudChipRenderer") {
			ep := jobj(chip["navigationEndpoint"])
			sel, _ := chip["isSelected"].(bool)
			chips = append(chips, filterChip{label: textOf(chip["text"]), selected: sel, endpoint: ep})
		}
		return chips
	}
	chips := collectNodes(page, "chipViewModel")
	if len(chips) == 0 {
		return []filterChip{}
	}
	first := chips[0]
	if dt := jstr(first["displayType"]); dropDownTypes[dt] {
		// Thanh dạng dropdown: đọc các ListItemView trong sheet của chip đầu.
		cmd := unwrapCommand(jget(first, "tapCommand"))
		sheet := jobj(jget(cmd, "showSheetCommand", "panelLoadingStrategy", "inlineContent", "sheetViewModel"))
		list := jobj(jget(sheet, "content", "listViewModel"))
		if sheet == nil || list == nil {
			return nil
		}
		var out []filterChip
		for _, item := range jarr(list["listItems"]) {
			it := jobj(jobj(item)["listItemViewModel"])
			if it == nil {
				continue
			}
			sel, _ := it["isSelected"].(bool)
			ep := jobj(jget(it, "rendererContext", "commandContext", "onTap"))
			out = append(out, filterChip{label: textOf(it["title"]), selected: sel, endpoint: ep})
		}
		return out
	}
	var out []filterChip
	for _, chip := range chips {
		sel, _ := chip["selected"].(bool)
		out = append(out, filterChip{label: jstr(chip["text"]), selected: sel, endpoint: jobj(chip["tapCommand"])})
	}
	return out
}

func isLatest(chip filterChip) bool {
	return strings.EqualFold(strings.TrimSpace(chip.label), "latest")
}

// latestVideosFeed trả feed tab Videos theo đúng thứ tự Mới nhất, chỉ gọi bộ lọc
// khi nó chưa được chọn. Mọi trạng thái không chứng minh được thứ tự Mới nhất là
// lỗi: danh sách Popular/Oldest tuyệt đối không được đọc như khám phá mới.
func (c *Client) latestVideosFeed(ctx context.Context, sess *session, tabPage jmap) (jmap, error) {
	chips := filterChips(tabPage)
	if chips == nil {
		return nil, newFetchError(KindUnexpectedPage,
			"Videos tab sort filters are unreadable; Latest order cannot be confirmed", 0, nil)
	}
	if len(chips) == 0 {
		return tabPage, nil
	}
	var latest *filterChip
	for i := range chips {
		if isLatest(chips[i]) {
			latest = &chips[i]
			break
		}
	}
	if latest == nil {
		return nil, newFetchError(KindUnexpectedPage, "Videos tab has no Latest sort filter", 0, nil)
	}
	if latest.selected {
		return tabPage, nil
	}
	if latest.endpoint == nil {
		return nil, newFetchError(KindUnexpectedPage, "Latest sort filter has no endpoint", 0, nil)
	}
	filtered, err := sess.callEndpoint(ctx, latest.endpoint)
	if err != nil {
		return nil, err
	}
	after := filterChips(filtered)
	if after == nil {
		return nil, newFetchError(KindUnexpectedPage, "YouTube did not apply the Latest sort filter", 0, nil)
	}
	if len(after) > 0 {
		ok := false
		for _, chip := range after {
			if isLatest(chip) && chip.selected {
				ok = true
				break
			}
		}
		if !ok {
			return nil, newFetchError(KindUnexpectedPage, "YouTube did not apply the Latest sort filter", 0, nil)
		}
	}
	return filtered, nil
}
