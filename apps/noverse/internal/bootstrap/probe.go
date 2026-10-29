package bootstrap

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"fmt"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
)

// ProbeModel 按配置构造一个模型客户端但不挂到任何角色上，只供查询能力
// （如可用推理强度）使用；构造本身不发网络请求。
func (c Config) ProbeModel(provider, model string) (llmtypes.ChatModel, error) {
	provider = strings.TrimSpace(provider)
	pc, ok := c.Providers[provider]
	if !ok {
		return nil, fmt.Errorf("provider %q chưa được cấu hình: %w", provider, errs.ErrConfig)
	}
	return createModelFromConfig(provider, strings.TrimSpace(model), pc, make(map[string]llmtypes.ChatModel))
}
