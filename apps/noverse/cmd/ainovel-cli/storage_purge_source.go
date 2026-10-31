package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

func runStorageMigration(args []string) error {
	if len(args) == 0 || args[0] != "purge-source" {
		return fmt.Errorf("cần lệnh storage migration purge-source <migration-id> --dry-run=true|false")
	}
	return runStoragePurgeSource(args[1:])
}

func parseSourcePurgeArgs(args []string) (string, bool, error) {
	if len(args) < 2 || strings.HasPrefix(args[0], "-") || strings.TrimSpace(args[0]) == "" {
		return "", false, fmt.Errorf("cần <migration-id> và cờ rõ ràng --dry-run=true hoặc --dry-run=false")
	}
	flags := flag.NewFlagSet("storage migration purge-source", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dryRun := flags.String("dry-run", "", "bắt buộc true (chỉ xem) hoặc false (xoá thật)")
	if err := flags.Parse(args[1:]); err != nil {
		return "", false, err
	}
	if flags.NArg() != 0 || (*dryRun != "true" && *dryRun != "false") {
		return "", false, fmt.Errorf("cần đúng --dry-run=true hoặc --dry-run=false; không có giá trị mặc định")
	}
	return args[0], *dryRun == "true", nil
}

func runStoragePurgeSource(args []string) error {
	id, dryRun, err := parseSourcePurgeArgs(args)
	if err != nil {
		return err
	}
	dsn := strings.TrimSpace(os.Getenv("MIGRATION_DATABASE_URL"))
	if dsn == "" {
		return fmt.Errorf("dọn kho nguồn yêu cầu MIGRATION_DATABASE_URL của vai trò bảo trì; không dùng DATABASE_URL của ứng dụng")
	}
	cfg, err := bootstrap.StorageConfigFromEnv()
	if err != nil {
		return err
	}
	keys, err := activeKeys()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	pool, err := postgres.Open(ctx, postgres.PoolConfig{DSN: dsn, MaxConns: cfg.MaxConns, ConnectTimeout: cfg.ConnectTimeout})
	if err != nil {
		return fmt.Errorf("mở kết nối bảo trì để dọn kho nguồn thất bại")
	}
	defer pool.Close()
	report, runErr := bootstrap.PurgeMigrationSource(ctx, pool, keys, id, dryRun)
	fmt.Print(report.Format())
	return runErr
}
