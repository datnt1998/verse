package audio

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

// PackDefaults chỉ áp dụng cho phân vai mới; từ điển luôn nằm dưới mục người dùng.
type PackDefaults struct {
	Mode       string          `json:"che_do"`
	Gender     string          `json:"giong"`
	Dictionary []Pronunciation `json:"-"`
}

type packSource interface {
	Section(string, any) (bool, error)
	ReadFile(string) ([]byte, error)
}

// ReadPackDefaults đọc audio và cach-doc.json từ cùng nguồn của gói thể loại.
// Người cao tuổi và nhóm hỗn hợp không có giọng mặc định trong Master §15.4.
func ReadPackDefaults(pack packSource, listener string) (*PackDefaults, error) {
	var defaults PackDefaults
	hasAudio, err := pack.Section("audio", &defaults)
	if err != nil {
		return nil, err
	}
	if hasAudio {
		if defaults.Mode != ModeSingle && defaults.Mode != ModeCast {
			return nil, fmt.Errorf("audio.che_do phải là single hoặc cast")
		}
		if defaults.Gender == "" {
			defaults.Gender = listenerGender(listener)
		} else if defaults.Gender != "nu" && defaults.Gender != "nam" {
			return nil, fmt.Errorf("audio.giong phải là nu hoặc nam")
		}
	}
	data, err := pack.ReadFile(pronounceFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		var dictionary pronunciationFile
		if err := json.Unmarshal(data, &dictionary); err != nil {
			return nil, fmt.Errorf("cach-doc.json của gói hỏng: %w", err)
		}
		defaults.Dictionary, err = NormalizePronunciations(dictionary.Entries)
		if err != nil {
			return nil, err
		}
	}
	if !hasAudio && len(defaults.Dictionary) == 0 {
		return nil, nil
	}
	return &defaults, nil
}

func listenerGender(listener string) string {
	switch strings.ToLower(strings.TrimSpace(listener)) {
	case "phụ nữ trung niên, nghe khi làm việc nhà", "nữ 18–35":
		return "nu"
	case "nam trung niên", "nam trẻ":
		return "nam"
	default:
		return ""
	}
}

// ensureCast chỉ chọn giọng khi chưa có vai.json. Danh mục có cả giọng dự án
// lẫn giọng engine; chọn ID nhỏ nhất để không phụ thuộc thứ tự engine trả về.
func (s *Service) ensureCast(cur *Cast, chars CharacterRef) (Cast, error) {
	defaults := CastDefaults{}
	if cur == nil && s.layout.Pack != nil {
		defaults.Mode = s.layout.Pack.Mode
		if gender := s.layout.Pack.Gender; gender != "" {
			voices, err := s.Voices(s.ctx)
			if err != nil {
				return Cast{}, err
			}
			for _, voice := range voices {
				if voice.ID != "" && normalizeGender(voice.Gender) == gender && (defaults.VoiceID == "" || voice.ID < defaults.VoiceID) {
					defaults.VoiceID = voice.ID
				}
			}
		}
	}
	return EnsureCast(cur, chars.Characters, chars.Support, defaults), nil
}
