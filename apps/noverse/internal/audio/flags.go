package audio

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// 听审标记（A5 "Cần nghe lại"）：用户边听边标"chỗ cần sửa"，导出前仍可决定。
// 落盘 audio/flags.json，与"段级 flags"（模型/规范化给出的提示）是两回事。

// Flag 是一条听审标记。
type Flag struct {
	ID          string    `json:"id"`
	Chapter     int       `json:"chapter"`
	SegmentID   string    `json:"segmentId,omitempty"`
	AtSec       float64   `json:"atSec"`
	Reason      string    `json:"reason"`
	Speaker     string    `json:"speaker,omitempty"`
	DurationSec float64   `json:"durationSec,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ReadFlags 读取标记；文件不存在返回空切片。
func ReadFlags(l Layout) ([]Flag, error) {
	data, err := os.ReadFile(l.flagsPath())
	if errors.Is(err, os.ErrNotExist) {
		return []Flag{}, nil
	}
	if err != nil {
		return nil, err
	}
	var flags []Flag
	if err := json.Unmarshal(data, &flags); err != nil {
		return nil, fmt.Errorf("flags.json hỏng: %w", err)
	}
	if flags == nil {
		return []Flag{}, nil
	}
	return flags, nil
}

// WriteFlags 原子写标记。
func WriteFlags(l Layout, flags []Flag) error {
	if flags == nil {
		flags = []Flag{}
	}
	if err := os.MkdirAll(l.AudioDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(flags, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(l.flagsPath(), data)
}

// AddFlag 追加一条标记（同一段同一理由重复标记视为同一条，不刷屏）。
func AddFlag(l Layout, f Flag) (Flag, error) {
	if f.Chapter <= 0 {
		return Flag{}, conflict("thiếu số chương")
	}
	f.Reason = strings.TrimSpace(f.Reason)
	if f.Reason == "" {
		return Flag{}, conflict("thiếu lý do cần nghe lại")
	}
	if f.AtSec < 0 {
		f.AtSec = 0
	}
	flags, err := ReadFlags(l)
	if err != nil {
		return Flag{}, err
	}
	for _, old := range flags {
		if old.Chapter == f.Chapter && old.SegmentID == f.SegmentID && strings.EqualFold(old.Reason, f.Reason) {
			return old, nil
		}
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}
	if f.ID == "" {
		f.ID = fmt.Sprintf("f%d", f.CreatedAt.UnixNano())
	}
	flags = append(flags, f)
	if err := WriteFlags(l, flags); err != nil {
		return Flag{}, err
	}
	return f, nil
}

// DeleteFlag 删除一条标记。
func DeleteFlag(l Layout, id string) error {
	flags, err := ReadFlags(l)
	if err != nil {
		return err
	}
	out := make([]Flag, 0, len(flags))
	found := false
	for _, f := range flags {
		if f.ID == id {
			found = true
			continue
		}
		out = append(out, f)
	}
	if !found {
		return ErrNotFound
	}
	return WriteFlags(l, out)
}

// ErrNotFound 表示请求的音频对象不存在（handler 映射 404）。
var ErrNotFound = errors.New("không tìm thấy")
