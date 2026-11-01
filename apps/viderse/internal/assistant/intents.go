package assistant

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// IntentID là tập intent DUY NHẤT trợ lý hiểu (Gói V11 + kế hoạch nhiều bước).
// Schema đầu ra của mô hình lấy enum từ đây và `run` từ chối mọi id khác, nên
// một câu không thể sinh ra thao tác ngoài danh sách (vd. xoá kênh).
type IntentID string

const (
	RedrawImage    IntentID = "redraw_image"
	RedrawAll      IntentID = "redraw_all"
	SkipImage      IntentID = "skip_image"
	EditMetadata   IntentID = "edit_metadata"
	RerollBackdrop IntentID = "reroll_backdrop"
	NoImages       IntentID = "no_images"
	Pause          IntentID = "pause"
	Resume         IntentID = "resume"
	SetPace        IntentID = "set_pace"
	FilterAuto     IntentID = "filter_auto"
	MakeLatest     IntentID = "make_latest"
	Prioritize     IntentID = "prioritize"
	SetAutonomy    IntentID = "set_autonomy"
	AddRule        IntentID = "add_rule"
	MakeVideos     IntentID = "make_videos"
)

type level int

const (
	jobLevel level = iota
	channelLevel
)

// Agent là mã trợ lý hiện trên thẻ (AgentId của hợp đồng dữ liệu §1).
const (
	agentScout    = "scout"
	agentDirector = "director"
	agentArtist   = "artist"
	agentQA       = "qa"
)

// intentSpec là một dòng của bảng intent. Bảng intent → service thật:
//
//	redraw_image    Images.Redraw            (Review op regenerate, nút “Vẽ lại”)
//	redraw_all      Images.RedrawAll         (Review “Vẽ lại tất cả”)
//	skip_image      Images.Review op skip    (Review “Bỏ ảnh này”)
//	edit_metadata   Metadata.Rewrite         (panel Tiêu đề/Mô tả/Tags “Viết lại”)
//	reroll_backdrop Backdrops.Edit op reroll (Nền › Đổi nền khác)
//	no_images       Images.ApplyOverrides noImages (Prompt ảnh › Không tạo ảnh)
//	pause / resume  Jobs.Pause / Jobs.Resume (⋯ Tạm dừng / Chạy tiếp)
//	set_pace        Channels.Update pacePerDay (Kênh › Nhịp)
//	filter_auto     Channels.SetSourceMinDuration (Kênh › Nguồn › Tự lấy)
//	make_latest     Queue.Enqueue            (Thêm video / Tự lấy pick)
//	make_videos     Queue.Enqueue            (như trên)
//	prioritize      Queue.Patch action front (Hàng chờ › Lên đầu)
//	set_autonomy    Autonomy.ApplyPreset     (Kênh › Mức tự chủ)
//	add_rule        Rules.AddRule            (Kênh › Quy tắc đã học)
type intentSpec struct {
	id    IntentID
	level level
	agent string
}

var registry = []intentSpec{
	{RedrawImage, jobLevel, agentArtist},
	{RedrawAll, jobLevel, agentArtist},
	{SkipImage, jobLevel, agentArtist},
	{EditMetadata, jobLevel, agentDirector},
	{RerollBackdrop, jobLevel, agentScout},
	{NoImages, jobLevel, agentArtist},
	{Pause, jobLevel, agentQA},
	{Resume, jobLevel, agentQA},
	{SetPace, channelLevel, agentScout},
	{FilterAuto, channelLevel, agentScout},
	{MakeLatest, channelLevel, agentScout},
	{Prioritize, channelLevel, agentScout},
	{SetAutonomy, channelLevel, agentQA},
	{AddRule, channelLevel, agentDirector},
	{MakeVideos, channelLevel, agentScout},
}

func specOf(id IntentID) (intentSpec, bool) {
	for _, s := range registry {
		if s.id == id {
			return s, true
		}
	}
	return intentSpec{}, false
}

// intentNames là enum đưa vào schema của mô hình.
func intentNames() []string {
	out := make([]string, len(registry))
	for i, s := range registry {
		out[i] = string(s.id)
	}
	return out
}

// Tham số của từng intent. Chỉ chứa giá trị ĐÃ phân giải bằng dữ liệu thật (id
// video, id kênh, id ảnh…); `run` giải mã nghiêm ngặt rồi kiểm lại với dữ liệu
// hiện tại nên client không đưa được id lạ qua `params`.
type (
	jobParams struct {
		JobID string `json:"jobId"`
	}
	sceneParams struct {
		JobID    string `json:"jobId"`
		ItemID   string `json:"itemId"`
		SceneNo  int    `json:"sceneNo"`
		Feedback string `json:"feedback,omitempty"`
	}
	redrawAllParams struct {
		JobID    string `json:"jobId"`
		Count    int    `json:"count"`
		Feedback string `json:"feedback,omitempty"`
	}
	metadataParams struct {
		JobID     string   `json:"jobId"`
		Parts     []string `json:"parts"`
		Direction string   `json:"direction,omitempty"`
	}
	paceParams struct {
		ChannelID string `json:"channelId"`
		PerDay    int    `json:"perDay"`
	}
	filterParams struct {
		ChannelID      string `json:"channelId"`
		RivalID        string `json:"rivalId"`
		MinDurationSec int    `json:"minDurationSec"`
	}
	makeParams struct {
		ChannelID string `json:"channelId"`
		RivalID   string `json:"rivalId"`
		VideoID   string `json:"videoId"`
		Title     string `json:"title"`
	}
	prioritizeParams struct {
		ChannelID   string `json:"channelId"`
		QueueItemID string `json:"queueItemId"`
		Title       string `json:"title"`
	}
	autonomyParams struct {
		ChannelID string `json:"channelId"`
		Preset    string `json:"preset"`
	}
	ruleParams struct {
		ChannelID string `json:"channelId"`
		Text      string `json:"text"`
	}
)

// decodeParams giải mã nghiêm ngặt (không trường lạ, không dữ liệu thừa).
func decodeParams[T any](raw json.RawMessage) (T, error) {
	var out T
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return out, fmt.Errorf("params không hợp lệ: %w", err)
	}
	if dec.More() {
		return out, fmt.Errorf("params có dữ liệu thừa")
	}
	return out, nil
}

func mustParams(p any) json.RawMessage {
	raw, err := json.Marshal(p)
	if err != nil {
		panic(err) // kiểu params cố định, không thể lỗi
	}
	return raw
}
