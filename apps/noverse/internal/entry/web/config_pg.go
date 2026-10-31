package web

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/identity"
)

func (g *Gateway) hostedConfigDocuments() (*postgres.ConfigDocuments, error) {
	capability, ok := g.libRepo.(interface {
		ConfigDocuments() *postgres.ConfigDocuments
	})
	if !ok {
		return nil, errs.ErrStorageUnsupported
	}
	return capability.ConfigDocuments(), nil
}

func configActor(user identity.User) *postgres.Actor {
	return &postgres.Actor{Username: user.Username, GlobalRole: string(user.Role)}
}

func (g *Gateway) loadTargetConfig(ctx context.Context, t configTarget, user identity.User) (bootstrap.Config, bool, error) {
	if !t.hosted {
		if _, err := os.Stat(t.path); err != nil {
			return bootstrap.Config{}, false, nil
		}
		cfg, err := bootstrap.LoadConfigFile(t.path)
		if err != nil {
			return bootstrap.Config{}, false, fmt.Errorf("tệp cấu hình hiện tại hỏng, không ghi đè: %w", err)
		}
		return cfg, true, nil
	}
	docs, err := g.hostedConfigDocuments()
	if err != nil {
		return bootstrap.Config{}, false, err
	}
	data, _, err := docs.Get(ctx, configActor(user), t.project, t.story.Story)
	if err != nil || data == nil {
		return bootstrap.Config{}, false, err
	}
	cfg, _, err := bootstrap.ParseConfigDocument(data)
	return cfg, true, err
}

func (g *Gateway) saveTargetConfig(ctx context.Context, t configTarget, user identity.User, cfg bootstrap.Config, expected string) error {
	if !t.hosted {
		return bootstrap.SaveConfig(t.path, cfg)
	}
	docs, err := g.hostedConfigDocuments()
	if err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return docs.Put(ctx, configActor(user), t.project, t.story.Story, data, expected)
}
