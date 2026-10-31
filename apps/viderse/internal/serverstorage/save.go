package serverstorage

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

type credentials struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
}

func (c *credentials) decode(raw []byte) error {
	if err := json.Unmarshal(raw, c); err != nil {
		return errors.New("Không đọc được cặp khoá đã lưu")
	}
	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		return errors.New("Cặp khoá đã lưu không hợp lệ")
	}
	return nil
}

// DecodeCredentials validates sealed payloads at the persistence boundary.
func DecodeCredentials(raw []byte) (string, string, error) {
	var pair credentials
	if err := pair.decode(raw); err != nil {
		return "", "", err
	}
	return pair.AccessKeyID, pair.SecretAccessKey, nil
}
func (s *Service) Save(ctx context.Context, actor string, c S3Config, testID string) (SaveResult, error) {
	if s.opts.EnvLocked {
		return SaveResult{}, &Error{403, "Cấu hình do máy chủ đặt"}
	}
	readiness, err := s.encryptionReadiness(ctx)
	if err != nil {
		return SaveResult{}, err
	}
	if !readiness.OK {
		return SaveResult{}, &Error{423, "Chưa dùng được khoá mã hoá cấu hình trên máy chủ"}
	}
	cfg, state, err := s.candidate(ctx, c)
	if err != nil {
		return SaveResult{}, err
	}
	s.mu.Lock()
	session := s.tests[testID]
	s.mu.Unlock()
	if session == nil {
		return SaveResult{}, conflict("Lượt kiểm tra đã hết hạn")
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session == nil || session.actor != actor || time.Now().After(session.expires) || !session.success || session.revision != state.Revision || session.fingerprint != configFingerprint(cfg) {
		return SaveResult{}, conflict("Cấu hình chưa được kiểm tra thành công hoặc đã thay đổi")
	}
	// A successful browser test is never transferable to a changed candidate,
	// another account, another saved revision, or a restarted server.
	raw, err := json.Marshal(credentials{cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey})
	if err != nil {
		return SaveResult{}, err
	}
	sealed, err := s.opts.Box.Seal(raw)
	if err != nil {
		return SaveResult{}, &Error{423, "Không niêm phong được cặp khoá truy cập"}
	}
	id, err := randomID()
	if err != nil {
		return SaveResult{}, err
	}
	current, err := s.opts.Repository.Descriptor(ctx, state.Active)
	if err != nil {
		return SaveResult{}, err
	}
	immediate := media.NamespaceID(current.Config) == media.NamespaceID(cfg)
	if _, err = s.opts.Repository.Configure(ctx, media.Descriptor{ID: id, Config: cfg, Credentials: sealed}, state.Revision, immediate); err != nil {
		return SaveResult{}, err
	}
	s.mu.Lock()
	delete(s.tests, testID)
	s.mu.Unlock()
	s.changed()
	if immediate {
		return SaveResult{NeedsMigration: nil}, nil
	}
	usage, err := s.opts.Repository.Usage(ctx)
	if err != nil {
		return SaveResult{}, err
	}
	need := &MigrationNeed{Groups: usage}
	for _, group := range usage {
		if group.Files != nil {
			need.Files += *group.Files
		}
		if group.Bytes != nil {
			need.Bytes += *group.Bytes
		}
	}
	return SaveResult{NeedsMigration: need}, nil
}
