package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/storagegc"
	"github.com/datnt1998/verse/apps/viderse/internal/storageimport"
)

func runStorage(args []string) error {
	if len(args) == 0 {
		return errors.New("cách dùng: viderse storage import|materialize|gc [--dry-run|--apply]")
	}
	action := args[0]
	if action != "import" && action != "materialize" && action != "gc" {
		return fmt.Errorf("lệnh lưu trữ không hợp lệ: %s", action)
	}
	flags := flag.NewFlagSet("storage "+action, flag.ContinueOnError)
	apply := flags.Bool("apply", false, "thực hiện sau khi đã xem trước")
	dryRun := flags.Bool("dry-run", true, "chỉ xem trước (mặc định)")
	target := ""
	store := ""
	grace := 24 * time.Hour
	if action == "materialize" {
		flags.StringVar(&target, "to", "", "thư mục đích tuyệt đối, không ghi đè nội dung khác")
	}
	if action == "gc" {
		flags.StringVar(&store, "store", "", "mã kho đã cấu hình; mặc định kho đang hoạt động")
		flags.DurationVar(&grace, "grace", 24*time.Hour, "thời gian chờ trước khi xoá, ít nhất một giờ")
	}
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("có tham số lưu trữ không nhận diện")
	}
	explicitDry := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "dry-run" {
			explicitDry = true
		}
	})
	if *apply && explicitDry && *dryRun {
		return errors.New("chọn --apply hoặc --dry-run, không dùng cả hai")
	}
	if action == "materialize" && target == "" {
		return errors.New("cần --to là thư mục khôi phục tuyệt đối")
	}
	ctx := context.Background()
	pool, err := openRivalPool()
	if err != nil {
		return err
	}
	defer pool.Close()
	root, err := cliProjectRoot(ctx, pool)
	if err != nil {
		return err
	}
	manager, err := newMediaRuntime(ctx, pool, func() string { return root }, nil, false, nil, false)
	if err != nil {
		return err
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := manager.Close(shutdown); err != nil {
			fmt.Fprintln(os.Stderr, "không đóng được kho dữ liệu:", err)
		}
	}()
	repo := postgres.NewBlobRepository(pool)
	// The CLI JSON envelope deliberately accepts the three distinct typed result
	// DTOs below; no untyped values enter the application service interfaces.
	var result any
	switch action {
	case "import":
		entries, err := manager.Inventory(ctx)
		if err != nil {
			return err
		}
		if *apply {
			if err := manager.ImportExisting(ctx); err != nil {
				return err
			}
		}
		var bytes int64
		for _, entry := range entries {
			bytes += entry.Bytes
		}
		result = struct {
			Files   int   `json:"files"`
			Bytes   int64 `json:"bytes"`
			Applied bool  `json:"applied"`
		}{len(entries), bytes, *apply}
	case "materialize":
		result, err = storageimport.Materialize(ctx, manager.Media(), target, *apply)
	case "gc":
		if store == "" {
			state, stateErr := repo.State(ctx)
			if stateErr != nil {
				return stateErr
			}
			store = state.Active
		}
		result, err = (storagegc.Service{Repository: repo, Media: manager.Media(), Grace: grace}).Run(ctx, store, *apply)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
