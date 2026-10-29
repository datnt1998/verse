// Package auth quản lý đăng nhập OAuth cho các provider dùng gói thuê bao
// (subscription) thay vì API Key — mô phỏng kiến trúc AuthStorage + OAuth
// registry của omp:
//
//   - Store: kho credential auth.json dưới thư mục dữ liệu của app (0600,
//     khoá file liên tiến trình); thư mục do Options.StoreDir quy định.
//   - Provider: định nghĩa luồng đăng nhập/refresh của từng nhà cung cấp, đăng ký
//     vào registry theo ID (vd. "openai-codex").
//   - Manager: cấp access token còn hạn cho lớp wire, tự refresh khi sắp hết hạn
//     hoặc khi server trả 401.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

// ErrNotLoggedIn báo provider chưa có credential trong kho.
var ErrNotLoggedIn = errors.New("chưa đăng nhập")

// Credential là bộ token OAuth của một provider.
type Credential struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh"`
	// Expires là thời điểm access token hết hạn (Unix mili-giây).
	Expires int64 `json:"expires"`
	// AccountID là workspace/tài khoản mà token rút hạn mức (vd. chatgpt_account_id).
	AccountID string `json:"account_id,omitempty"`
	Email     string `json:"email,omitempty"`
	Plan      string `json:"plan,omitempty"`
	// OrgID/OrgName: tổ chức (workspace thuê bao) mà token gắn vào — một email có thể
	// giữ nhiều gói (vd. ghế Team + gói Max cá nhân). Cố định từ lúc đăng nhập (Anthropic).
	OrgID   string `json:"org_id,omitempty"`
	OrgName string `json:"org_name,omitempty"`
}

// ExpiresAt trả về thời điểm hết hạn dạng time.Time.
func (c Credential) ExpiresAt() time.Time { return time.UnixMilli(c.Expires) }

// Label là mô tả ngắn để hiển thị: "email · tổ chức (plan)". Tên tổ chức bị bỏ khi
// nó đã chứa email (Anthropic hay đặt "<email>'s Organization").
func (c Credential) Label() string {
	label := c.Email
	if label == "" {
		label = c.AccountID
	}
	if c.OrgName != "" && !(c.Email != "" && strings.Contains(strings.ToLower(c.OrgName), strings.ToLower(c.Email))) {
		if label == "" {
			label = c.OrgName
		} else {
			label += " · " + c.OrgName
		}
	}
	if c.Plan != "" {
		if label == "" {
			return c.Plan
		}
		label += " (" + c.Plan + ")"
	}
	return label
}

// Store là kho credential dạng JSON {"<provider>": Credential}. Mọi thao tác
// đọc-sửa-ghi đều nằm dưới khoá file để hai tiến trình cùng app không ghi đè
// refresh token của nhau (refresh token của OpenAI xoay vòng: dùng lại token cũ
// sẽ bị thu hồi).
type Store struct {
	path string
}

// NewStore tạo kho tại đường dẫn chỉ định.
func NewStore(path string) *Store { return &Store{path: path} }

// DefaultStorePath trả về $HOME/<Options.StoreDir>/auth.json. Lỗi khi app chưa
// Configure — không có đường mặc định ngầm để tránh hai app ghi đè kho của nhau.
func DefaultStorePath() (string, error) {
	o := options()
	if o.StoreDir == "" {
		return "", errors.New("auth: chưa Configure — không xác định được thư mục kho credential")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("không xác định được thư mục home: %w", err)
	}
	return filepath.Join(home, o.StoreDir, "auth.json"), nil
}

// DefaultStore mở kho mặc định tại DefaultStorePath.
func DefaultStore() (*Store, error) {
	path, err := DefaultStorePath()
	if err != nil {
		return nil, err
	}
	return NewStore(path), nil
}

// Path trả về đường dẫn file kho.
func (s *Store) Path() string { return s.path }

// Get đọc credential của provider; ok=false khi chưa đăng nhập.
func (s *Store) Get(provider string) (cred Credential, ok bool, err error) {
	err = s.withLock(func() error {
		all, err := s.readAll()
		if err != nil {
			return err
		}
		cred, ok = all[provider]
		return nil
	})
	return cred, ok, err
}

// Put ghi (thay thế) credential của provider.
func (s *Store) Put(provider string, cred Credential) error {
	return s.Update(provider, func(Credential, bool) (Credential, bool, error) {
		return cred, true, nil
	})
}

// Delete xoá credential; trả về true nếu trước đó có tồn tại.
func (s *Store) Delete(provider string) (existed bool, err error) {
	err = s.Update(provider, func(_ Credential, ok bool) (Credential, bool, error) {
		existed = ok
		return Credential{}, false, nil
	})
	return existed, err
}

// Update đọc-sửa-ghi credential của provider trong một khoá file. fn nhận giá
// trị hiện tại và trả về giá trị mới cùng keep=false để xoá mục.
func (s *Store) Update(provider string, fn func(current Credential, ok bool) (next Credential, keep bool, err error)) error {
	return s.withLock(func() error {
		all, err := s.readAll()
		if err != nil {
			return err
		}
		current, ok := all[provider]
		next, keep, err := fn(current, ok)
		if err != nil {
			return err
		}
		if keep {
			all[provider] = next
		} else {
			if !ok {
				return nil
			}
			delete(all, provider)
		}
		return s.writeAll(all)
	})
}

func (s *Store) withLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("tạo thư mục kho credential: %w", err)
	}
	lock := flock.New(s.path+".lock", flock.SetPermissions(0o600))
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("khoá kho credential: %w", err)
	}
	defer lock.Unlock()
	return fn()
}

func (s *Store) readAll() (map[string]Credential, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Credential{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("đọc kho credential %s: %w", s.path, err)
	}
	all := map[string]Credential{}
	if strings.TrimSpace(string(data)) == "" {
		return all, nil
	}
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, fmt.Errorf("kho credential %s bị hỏng: %w", s.path, err)
	}
	return all, nil
}

// writeAll ghi nguyên tử (file tạm + rename) với quyền 0600.
func (s *Store) writeAll(all map[string]Credential) error {
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("mã hoá kho credential: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".auth-*.json")
	if err != nil {
		return fmt.Errorf("tạo file tạm cho kho credential: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("đặt quyền kho credential: %w", err)
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("ghi kho credential: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("ghi kho credential: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("lưu kho credential: %w", err)
	}
	return nil
}
