package youtube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"time"
	"unicode/utf8"
)

const (
	// MaxCursorBytes là trần kích thước một cursor tuần tự hoá để lưu trữ.
	MaxCursorBytes = 64 * 1024
	// MaxRemainderVideos / MaxRemainderBytes là trần của phần dư một trang lưu kèm cursor.
	MaxRemainderVideos = 500
	MaxRemainderBytes  = 512 * 1024
)

var printableRe = regexp.MustCompile(`^[\x21-\x7e]+$`)
var scannedVideoIDRe = regexp.MustCompile(`^[\w-]{11}$`)
var cursorSessionNameRe = regexp.MustCompile(`^[A-Z0-9_]{1,40}$`)
var cursorSessionVerRe = regexp.MustCompile(`^[\w.-]{1,64}$`)

// Continuation là phần payload duy nhất được lưu từ endpoint: token browse.
type Continuation struct {
	Token   string `json:"token"`
	Request string `json:"request"`
}

// UnmarshalJSON bắt buộc đúng hai khoá token/request dạng chuỗi (schema strict
// của bản TS: thiếu khoá hay khoá lạ đều là dữ liệu hỏng).
func (c *Continuation) UnmarshalJSON(data []byte) error {
	if !hasExactKeys(data, "token", "request") {
		return errors.New("continuation keys mismatch")
	}
	type alias Continuation
	var a alias
	if err := strictUnmarshal(data, &a); err != nil {
		return err
	}
	*c = Continuation(a)
	return nil
}

// SessionContext ghi lại môi trường client đã phát hành cursor: một cursor chỉ
// phát lại được trên session cùng client name/version và visitor data.
type SessionContext struct {
	ClientName    string `json:"clientName"`
	ClientVersion string `json:"clientVersion"`
	// VisitorData rỗng nghĩa là cursor do session chia sẻ phát hành.
	VisitorData string `json:"visitorData"`
}

// MarshalJSON để visitorData luôn null thay vì "" khi session chia sẻ phát hành.
func (s SessionContext) MarshalJSON() ([]byte, error) {
	type wire struct {
		ClientName    string  `json:"clientName"`
		ClientVersion string  `json:"clientVersion"`
		VisitorData   *string `json:"visitorData"`
	}
	var vd *string
	if s.VisitorData != "" {
		vd = &s.VisitorData
	}
	return json.Marshal(wire{s.ClientName, s.ClientVersion, vd})
}

// UnmarshalJSON bắt buộc đúng ba khoá; visitorData phải có mặt và là null
// hoặc chuỗi (schema nullable bắt buộc, không phải optional).
func (s *SessionContext) UnmarshalJSON(data []byte) error {
	if !hasExactKeys(data, "clientName", "clientVersion", "visitorData") {
		return errors.New("session keys mismatch")
	}
	type wire struct {
		ClientName    string           `json:"clientName"`
		ClientVersion string           `json:"clientVersion"`
		VisitorData   *json.RawMessage `json:"visitorData"`
	}
	var w wire
	if err := strictUnmarshal(data, &w); err != nil {
		return err
	}
	s.ClientName = w.ClientName
	s.ClientVersion = w.ClientVersion
	s.VisitorData = ""
	if w.VisitorData != nil {
		var vd *string
		if err := json.Unmarshal(*w.VisitorData, &vd); err != nil {
			return err
		}
		if vd != nil {
			s.VisitorData = *vd
		}
	}
	return nil
}

// ChannelCursor là trạng thái phân trang tối thiểu để đọc tiếp trang kênh sau
// khi lưu trữ: chỉ chứa token continuation + bối cảnh session, không object nào khác.
type ChannelCursor struct {
	Version      int            `json:"version"`
	ChannelID    string         `json:"channelId"`
	Tab          string         `json:"tab"`
	Order        string         `json:"order"`
	Continuation Continuation   `json:"continuation"`
	Session      SessionContext `json:"session"`
}

// UnmarshalJSON bắt buộc đúng sáu khoá cấp trên (nested tự kiểm qua
// UnmarshalJSON của chúng).
func (c *ChannelCursor) UnmarshalJSON(data []byte) error {
	if !hasExactKeys(data, "version", "channelId", "tab", "order", "continuation", "session") {
		return errors.New("cursor keys mismatch")
	}
	type alias ChannelCursor
	var a alias
	if err := strictUnmarshal(data, &a); err != nil {
		return err
	}
	*c = ChannelCursor(a)
	return nil
}

// ChannelVideoPage là một trang video của kênh cùng cursor sang trang kế.
type ChannelVideoPage struct {
	Videos     []ScannedVideo
	NextCursor *ChannelCursor
	// Profile chỉ có trên trang đầu.
	Profile *ChannelProfile
}

func invalidCursorErr(msg string) *FetchError {
	return newFetchError(KindCursorInvalid, msg, 0, nil)
}

// validateCursor kiểm hình dạng cursor; mọi sai lệch đều là cursor_invalid.
func validateCursor(c *ChannelCursor) error {
	if c.Version != 1 || !IsChannelID(c.ChannelID) || c.Tab != "videos" || c.Order != "latest" {
		return errors.New("cursor shape mismatch")
	}
	tok := c.Continuation.Token
	if tok == "" || len(tok) > 16_384 || !printableRe.MatchString(tok) {
		return errors.New("bad continuation token")
	}
	if c.Continuation.Request != "CONTINUATION_REQUEST_TYPE_BROWSE" {
		return errors.New("bad continuation request type")
	}
	if !cursorSessionNameRe.MatchString(c.Session.ClientName) {
		return errors.New("bad client name")
	}
	if !cursorSessionVerRe.MatchString(c.Session.ClientVersion) {
		return errors.New("bad client version")
	}
	if len(c.Session.VisitorData) > 1024 {
		return errors.New("oversized visitor data")
	}
	if c.Session.VisitorData != "" && !printableRe.MatchString(c.Session.VisitorData) {
		return errors.New("bad visitor data")
	}
	return nil
}

// strictUnmarshal là json.Unmarshal + DisallowUnknownFields + bắt buộc đọc
// hết input (giá trị JSON thứ hai phải là io.EOF): tương đương strictObject của
// schema bên TS — khoá lạ, dữ liệu thừa hay cú pháp sót đều là dữ liệu hỏng.
func strictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("trailing data")
	}
	return nil
}

// hasExactKeys báo object JSON có đúng bộ khoá yêu cầu (thiếu hay thừa đều false).
func hasExactKeys(data []byte, keys ...string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil || m == nil {
		return false
	}
	if len(m) != len(keys) {
		return false
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

// SerializeChannelCursor tuần tự cursor sau khi kiểm; lỗi khi cursor không hợp lệ
// hoặc JSON hoá vượt MaxCursorBytes.
func SerializeChannelCursor(c ChannelCursor) (string, error) {
	if err := validateCursor(&c); err != nil {
		return "", err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	if len(data) > MaxCursorBytes {
		return "", errors.New("cursor too large")
	}
	return string(data), nil
}

// ParseChannelCursor đọc cursor đã lưu; JSON hỏng, quá lớn hoặc sai hình dạng
// đều là cursor_invalid.
func ParseChannelCursor(s string) (ChannelCursor, error) {
	var cursor ChannelCursor
	invalid := invalidCursorErr("Stored cursor is corrupt or incompatible")
	if len(s) > MaxCursorBytes {
		return cursor, invalid
	}
	if err := strictUnmarshal([]byte(s), &cursor); err != nil {
		return cursor, invalid
	}
	if err := validateCursor(&cursor); err != nil {
		return cursor, invalid
	}
	return cursor, nil
}

// scannedVideoWire là hình lưu trữ của ScannedVideo: giống domain nhưng bắt
// buộc đúng khoá để phát hiện remainder lạ.
type scannedVideoWire struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	ThumbnailURL  string     `json:"thumbnailUrl"`
	DurationSec   *int64     `json:"durationSec"`
	ViewCount     *int64     `json:"viewCount"`
	PublishedText *string    `json:"publishedText"`
	PublishedAt   *time.Time `json:"publishedAt"`
}

func validateScannedVideo(v *ScannedVideo) error {
	if !scannedVideoIDRe.MatchString(v.ID) {
		return errors.New("bad video id")
	}
	if utf8.RuneCountInString(v.Title) > maxTitleLength {
		return errors.New("title too long")
	}
	if len(v.ThumbnailURL) > 2048 {
		return errors.New("thumbnail url too long")
	}
	if u, err := url.Parse(v.ThumbnailURL); err != nil || !u.IsAbs() {
		return errors.New("thumbnail url invalid")
	}
	if v.DurationSec != nil && *v.DurationSec < 0 {
		return errors.New("negative duration")
	}
	if v.ViewCount != nil && *v.ViewCount < 0 {
		return errors.New("negative view count")
	}
	if v.PublishedText != nil && utf8.RuneCountInString(*v.PublishedText) > maxPublishedTextLength {
		return errors.New("published text too long")
	}
	return nil
}

// SerializeScannedVideos tuần tự phần dư một trang (tối đa MaxRemainderVideos dòng).
func SerializeScannedVideos(videos []ScannedVideo) (string, error) {
	if len(videos) > MaxRemainderVideos {
		return "", fmt.Errorf("too many remainder videos: %d > %d", len(videos), MaxRemainderVideos)
	}
	for i := range videos {
		if err := validateScannedVideo(&videos[i]); err != nil {
			return "", err
		}
	}
	wire := make([]scannedVideoWire, len(videos))
	for i, v := range videos {
		wire[i] = scannedVideoWire(v)
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParseScannedVideos đọc phần dư đã lưu; JSON hỏng/quá lớn/sai hình dạng là cursor_invalid.
func ParseScannedVideos(s string) ([]ScannedVideo, error) {
	invalid := invalidCursorErr("Stored page remainder is corrupt")
	if len(s) > MaxRemainderBytes {
		return nil, invalid
	}
	// Phần dư bắt buộc là một mảng thật (schema mảng strict, không nullable);
	// mỗi phần tử phải có đúng bộ khoá — thiếu khoá nullable vẫn là dữ liệu hỏng.
	var elems []json.RawMessage
	dec := json.NewDecoder(bytes.NewReader([]byte(s)))
	if err := dec.Decode(&elems); err != nil || elems == nil {
		return nil, invalid
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, invalid
	}
	if len(elems) > MaxRemainderVideos {
		return nil, invalid
	}
	videos := make([]ScannedVideo, len(elems))
	for i, rawElem := range elems {
		if !hasExactKeys(rawElem, "id", "title", "thumbnailUrl", "durationSec",
			"viewCount", "publishedText", "publishedAt") {
			return nil, invalid
		}
		var w scannedVideoWire
		if err := strictUnmarshal(rawElem, &w); err != nil {
			return nil, invalid
		}
		videos[i] = ScannedVideo(w)
		if err := validateScannedVideo(&videos[i]); err != nil {
			return nil, invalid
		}
	}
	return videos, nil
}
