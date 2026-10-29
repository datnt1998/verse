package store

import (
	"os"

	"github.com/datnt1998/noverse/internal/domain"
)

// UsageStore 持久化 token / cost 累计用量到 meta/usage.json。
// 写入走 IO 的 atomic write（tmp + rename），Save 路径每次完整覆盖整个 state。
type fileUsageStore struct{ io *IO }

func newFileUsageStore(io *IO) *fileUsageStore { return &fileUsageStore{io: io} }

// Load 读取 usage.json。文件不存在或 schema 版本不匹配时返回 (nil, nil)，
// 由调用方决定是否走 session replay 一次性回填。
func (s *fileUsageStore) Load() (*domain.UsageState, error) {
	var state domain.UsageState
	if err := s.io.ReadJSON("meta/usage.json", &state); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if state.Schema != domain.UsageSchemaVersion {
		return nil, nil
	}
	return &state, nil
}

// Save 把 state 完整覆盖落盘。调用方负责 debounce / 节流。
func (s *fileUsageStore) Save(state domain.UsageState) error {
	state.Schema = domain.UsageSchemaVersion
	return s.io.WriteJSON("meta/usage.json", state)
}
