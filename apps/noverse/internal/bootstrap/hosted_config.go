package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/core/llmconfig"
)

// LoadHostedConfigChain uses the server's global credentials/config as the base
// and repository documents for project/story overrides. It never discovers a
// project directory or reads worker scratch as a configuration authority.
func LoadHostedConfigChain(ctx context.Context, docs *postgres.ConfigDocuments, actor *postgres.Actor, project, story string) ([]ConfigLevel, Config, error) {
	levels := []ConfigLevel{}
	var effective Config
	if path := DefaultConfigPath(); path != "" {
		level := ConfigLevel{Scope: "global", Path: path}
		cfg, found, err := loadOptionalJSON(path)
		if err != nil {
			slog.Warn("phân tích cấu hình toàn cục thất bại, đã bỏ qua (có thể bị cấp dự án/truyện ghi đè)", "module", "config", "path", path, "err", err)
		} else if found {
			level.Exists, level.Config, effective = true, cfg, cfg
			effective.Style = ""
		}
		levels = append(levels, level)
	}
	scopes := []string{"project"}
	if story != "" {
		scopes = append(scopes, "story")
	}
	for _, scope := range scopes {
		id := ""
		if scope == "story" {
			id = story
		}
		data, _, err := docs.Get(ctx, actor, project, id)
		if err != nil {
			return nil, Config{}, err
		}
		level := ConfigLevel{Scope: scope, Raw: map[string]any{}}
		if data != nil {
			cfg, raw, err := ParseConfigDocument(data)
			if err != nil {
				return nil, Config{}, fmt.Errorf("phân tích cấu hình cấp %s thất bại: %w", scopeLabel(scope), err)
			}
			level.Exists, level.Config, level.Raw = true, cfg, raw
			effective = MergeConfig(effective, HostedLayerConfig(cfg))
		}
		levels = append(levels, level)
	}
	return levels, effective, nil
}

// HostedLayerConfig is a project/story layer as the hosted merge sees it. The
// provider key and OAuth token live only in the global layer, so a lower layer
// must not decide where they are sent: its provider type/base_url/extra and
// tts.endpoint are dropped. The config API refuses to write them too, but
// layer documents can reach the repository through other writers, so the
// merge enforces the invariant itself. Everything else still overrides.
func HostedLayerConfig(cfg Config) Config {
	if len(cfg.Providers) > 0 {
		providers := make(map[string]llmconfig.ProviderConfig, len(cfg.Providers))
		for name, pc := range cfg.Providers {
			pc.Type, pc.BaseURL, pc.Extra = "", "", nil
			providers[name] = pc
		}
		cfg.Providers = providers
	}
	cfg.TTS.Endpoint = ""
	return cfg
}

// LoadHostedConfig is the engine-side form of the same HTTP inheritance chain.
func LoadHostedConfig(ctx context.Context, docs *postgres.ConfigDocuments, project, story string) (Config, error) {
	_, cfg, err := LoadHostedConfigChain(ctx, docs, nil, project, story)
	return cfg, err
}
