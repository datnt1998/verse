package metadata

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

type Content struct {
	Title       string                     `json:"title"`
	Description string                     `json:"description"`
	Tags        []string                   `json:"tags"`
	Reason      map[string]json.RawMessage `json:"reason,omitempty"`
	Edited      map[string]bool            `json:"edited,omitempty"`
	EditedBy    string                     `json:"editedBy,omitempty"`
	EditedAt    string                     `json:"editedAt,omitempty"`
}
type Version struct {
	At    string   `json:"at"`
	By    string   `json:"by"`
	Parts []string `json:"parts"`
	Content
}
type State struct {
	Revision int       `json:"revision"`
	Current  Content   `json:"current"`
	History  []Version `json:"history"`
}
type Patch struct {
	ExpectedRevision *int      `json:"expectedRevision"`
	Title            *string   `json:"title"`
	Description      *string   `json:"description"`
	Tags             *[]string `json:"tags"`
}
type Rewrite struct {
	ExpectedRevision *int     `json:"expectedRevision"`
	Parts            []string `json:"parts"`
	Direction        string   `json:"direction"`
	Feedback         string   `json:"feedback"`
}
type Restore struct {
	ExpectedRevision *int   `json:"expectedRevision"`
	At               string `json:"at"`
}

func invalid(msg string) error {
	return &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: msg}
}
func conflict() error {
	return &domain.CloneError{Code: domain.CloneErrConflict, Message: "Nội dung đã thay đổi — tải lại rồi thử lại"}
}
func (s *State) check(rev *int) error {
	if rev == nil || *rev != s.Revision {
		return conflict()
	}
	return nil
}
func validate(c Content) error {
	if n := utf8.RuneCountInString(c.Title); n < 1 || n > 100 {
		return invalid("Tiêu đề phải có từ 1 đến 100 ký tự")
	}
	if n := utf8.RuneCountInString(c.Description); n < 1 || n > 5000 {
		return invalid("Mô tả phải có từ 1 đến 5000 ký tự")
	}
	if len(c.Tags) > 30 || utf8.RuneCountInString(strings.Join(c.Tags, ",")) > 500 {
		return invalid("Tag vượt giới hạn")
	}
	for _, tag := range c.Tags {
		if tag == "" {
			return invalid("Tag không được trống")
		}
	}
	return nil
}
func validParts(parts []string) error {
	seen := map[string]bool{}
	if len(parts) == 0 {
		return invalid("Chọn phần cần viết lại")
	}
	for _, p := range parts {
		if (p != "title" && p != "description" && p != "tags") || seen[p] {
			return invalid("Phần nội dung không hợp lệ")
		}
		seen[p] = true
	}
	return nil
}
func merge(old, next Content, parts []string) Content {
	reasons := make(map[string]json.RawMessage, len(old.Reason))
	for part, reason := range old.Reason {
		reasons[part] = reason
	}
	old.Reason = reasons
	for _, p := range parts {
		switch p {
		case "title":
			old.Title = next.Title
		case "description":
			old.Description = next.Description
		case "tags":
			old.Tags = next.Tags
		}
		if reason, ok := next.Reason[p]; ok {
			old.Reason[p] = reason
		} else {
			delete(old.Reason, p)
		}
	}
	return old
}
func (s *State) save(next Content, by string, parts []string) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	previous := s.Current
	s.History = append([]Version{{At: now, By: previous.EditedBy, Parts: append([]string(nil), parts...), Content: previous}}, s.History...)
	if s.History[0].By == "" {
		s.History[0].By = "director"
	}
	if len(s.History) > 10 {
		s.History = s.History[:10]
	}
	next.EditedBy = by
	next.EditedAt = now
	s.Current = next
	s.Revision++
}
func (s *State) patch(p Patch) error {
	if err := s.check(p.ExpectedRevision); err != nil {
		return err
	}
	next := s.Current
	parts := []string{}
	if p.Title != nil {
		next.Title = *p.Title
		parts = append(parts, "title")
	}
	if p.Description != nil {
		next.Description = *p.Description
		parts = append(parts, "description")
	}
	if p.Tags != nil {
		next.Tags = *p.Tags
		parts = append(parts, "tags")
	}
	if err := validParts(parts); err != nil {
		return err
	}
	if err := validate(next); err != nil {
		return err
	}
	reasons := make(map[string]json.RawMessage, len(next.Reason))
	for part, reason := range next.Reason {
		reasons[part] = reason
	}
	for _, part := range parts {
		delete(reasons, part)
	}
	next.Reason = reasons
	edited := map[string]bool{}
	for k, v := range next.Edited {
		edited[k] = v
	}
	for _, p := range parts {
		edited[p] = true
	}
	next.Edited = edited
	s.save(next, "owner", parts)
	return nil
}
