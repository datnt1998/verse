package activity

import (
	"context"
	"fmt"
	"time"
)

type NotificationChannels struct {
	InApp  bool `json:"inApp"`
	System bool `json:"system"`
}
type QuietHours struct {
	Enabled bool   `json:"enabled"`
	Start   string `json:"start"`
	End     string `json:"end"`
}
type NotificationSettings struct {
	Kinds         map[string]NotificationChannels `json:"kinds"`
	DoNotDisturb  QuietHours                      `json:"doNotDisturb"`
	Badge         bool                            `json:"badge"`
	Merge         bool                            `json:"merge"`
	RecapOnReturn bool                            `json:"recapOnReturn"`
}

var notificationKinds = []string{"review_ready", "needs_attention", "ready_to_post", "task_done", "action_result"}

func DefaultNotificationSettings() NotificationSettings {
	s := NotificationSettings{Kinds: make(map[string]NotificationChannels), DoNotDisturb: QuietHours{Start: "23:00", End: "07:00"}, Badge: true, Merge: true, RecapOnReturn: true}
	for _, k := range notificationKinds {
		s.Kinds[k] = NotificationChannels{InApp: true, System: k == "review_ready" || k == "needs_attention" || k == "ready_to_post"}
	}
	return s
}
func minuteOfDay(v string) (int, error) {
	t, err := time.Parse("15:04", v)
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}
func (s NotificationSettings) Validate() error {
	if len(s.Kinds) != len(notificationKinds) {
		return fmt.Errorf("loại thông báo không hợp lệ")
	}
	for _, k := range notificationKinds {
		if _, ok := s.Kinds[k]; !ok {
			return fmt.Errorf("thiếu loại thông báo %s", k)
		}
	}
	if _, err := minuteOfDay(s.DoNotDisturb.Start); err != nil {
		return fmt.Errorf("giờ bắt đầu không hợp lệ")
	}
	if _, err := minuteOfDay(s.DoNotDisturb.End); err != nil {
		return fmt.Errorf("giờ kết thúc không hợp lệ")
	}
	return nil
}
func (s NotificationSettings) SystemAllowed(kind string, at time.Time) bool {
	if !s.Kinds[kind].System {
		return false
	}
	d := s.DoNotDisturb
	if !d.Enabled {
		return true
	}
	start, e1 := minuteOfDay(d.Start)
	end, e2 := minuteOfDay(d.End)
	if e1 != nil || e2 != nil {
		return false
	}
	now := at.Hour()*60 + at.Minute()
	if start == end {
		return false
	}
	if start < end {
		return now < start || now >= end
	}
	return now < start && now >= end
}

type settingsStore interface {
	Get(context.Context, string, any) (bool, error)
	Set(context.Context, string, any) error
}

const notificationSettingsKey = "notifications.preferences"

func (s *Service) NotificationSettings(ctx context.Context) (NotificationSettings, error) {
	prefs := DefaultNotificationSettings()
	store, ok := s.store.(settingsStore)
	if !ok {
		return prefs, nil
	}
	_, err := store.Get(ctx, notificationSettingsKey, &prefs)
	return prefs, err
}
func (s *Service) SetNotificationSettings(ctx context.Context, prefs NotificationSettings) error {
	if err := prefs.Validate(); err != nil {
		return err
	}
	store, ok := s.store.(settingsStore)
	if !ok {
		return fmt.Errorf("không lưu được cài đặt thông báo")
	}
	if err := store.Set(ctx, notificationSettingsKey, prefs); err != nil {
		return err
	}
	s.emit("")
	return nil
}

func (s *Service) Notifications(ctx context.Context) ([]Notification, error) {
	store, ok := s.store.(interface {
		ListNotifications(context.Context, int) ([]Notification, error)
	})
	if !ok {
		return s.store.ListUnreadNotifications(ctx, 50)
	}
	return store.ListNotifications(ctx, 100)
}
