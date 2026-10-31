package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

func runStorageSettings(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("dùng: ainovel-cli storage settings adopt-env | backfill-usage [--dry-run]")
	}
	if args[0] != "adopt-env" && args[0] != "backfill-usage" {
		return fmt.Errorf("lệnh cấu hình kho không hợp lệ")
	}
	fs := flag.NewFlagSet("storage settings "+args[0], flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "chỉ xem số hàng cần phân loại, không ghi DB")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("không nhận đối số bổ sung")
	}
	if args[0] == "adopt-env" && *dryRun {
		return fmt.Errorf("adopt-env là bước tiếp nhận tường minh; không hỗ trợ --dry-run")
	}
	cfg, err := bootstrap.StorageConfigFromEnv()
	if err != nil {
		return err
	}
	dsn := strings.TrimSpace(os.Getenv("MIGRATION_DATABASE_URL"))
	if cfg.Driver != bootstrap.StoreDriverPostgres || dsn == "" {
		return fmt.Errorf("yêu cầu STORE_DRIVER=postgres và MIGRATION_DATABASE_URL của vai trò bảo trì")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	pool, err := postgres.Open(ctx, postgres.PoolConfig{DSN: dsn, MaxConns: cfg.MaxConns, ConnectTimeout: cfg.ConnectTimeout})
	if err != nil {
		return fmt.Errorf("không mở được cơ sở dữ liệu bảo trì")
	}
	defer pool.Close()
	if args[0] == "adopt-env" {
		keys, err := activeKeys()
		if err != nil {
			return err
		}
		result, err := bootstrap.AdoptBlobEnv(ctx, pool, keys)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	if _, err := bootstrap.ResolveBlobConfig(ctx, pool); err != nil {
		return err
	}
	count, err := bootstrap.BackfillStorageUsage(ctx, pool, *dryRun)
	if err != nil {
		return fmt.Errorf("chưa phân loại xong registry kho tệp")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"dryRun": *dryRun, "count": count})
}
