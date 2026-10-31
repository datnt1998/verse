package activity

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidCursor = errors.New("con trỏ thông báo không hợp lệ")

type NotificationCursor struct {
	CreatedAt time.Time `json:"at"`
	ID        string    `json:"id"`
}

const notificationPageSize = 100

func decodeCursor(raw string) (*NotificationCursor, error) {
	if raw == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(data) > 256 {
		return nil, ErrInvalidCursor
	}
	var cursor NotificationCursor
	if json.Unmarshal(data, &cursor) != nil || cursor.CreatedAt.IsZero() || len(cursor.ID) != 36 || cursor.ID[8] != '-' || cursor.ID[13] != '-' || cursor.ID[18] != '-' || cursor.ID[23] != '-' {
		return nil, ErrInvalidCursor
	}
	id := strings.ReplaceAll(cursor.ID, "-", "")
	if len(id) != 32 {
		return nil, ErrInvalidCursor
	}
	if _, err := hex.DecodeString(id); err != nil {
		return nil, ErrInvalidCursor
	}
	return &cursor, nil
}
func (s *Service) NotificationPage(ctx context.Context, before string) ([]Notification, string, error) {
	cursor, err := decodeCursor(before)
	if err != nil {
		return nil, "", err
	}
	store, ok := s.store.(interface {
		ListNotificationsBefore(context.Context, *NotificationCursor, int) ([]Notification, error)
	})
	if !ok {
		return nil, "", errors.New("kho thông báo không hỗ trợ phân trang")
	}
	rows, err := store.ListNotificationsBefore(ctx, cursor, notificationPageSize+1)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) > notificationPageSize {
		rows = rows[:notificationPageSize]
		last := rows[len(rows)-1]
		raw, _ := json.Marshal(NotificationCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	return rows, next, nil
}
