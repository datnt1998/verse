package main

import (
	"context"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/scan"
	"github.com/jackc/pgx/v5/pgxpool"
)

// activityScanSchedule uses the same configured interval as scan.Status but
// reads all scan timestamps in one query rather than loading N scan snapshots.
func activityScanSchedule(pool *pgxpool.Pool, svc *scan.Service) func(context.Context) ([]time.Time, error) {
	return func(ctx context.Context) ([]time.Time, error) {
		interval := svc.RescanInterval(ctx)
		rows, err := pool.Query(ctx, `SELECT scanned_at FROM rival_channels WHERE scanned_at IS NOT NULL ORDER BY scanned_at`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var dates []time.Time
		for rows.Next() {
			var scanned time.Time
			if err := rows.Scan(&scanned); err != nil {
				return nil, err
			}
			dates = append(dates, scanned.Add(interval))
		}
		return dates, rows.Err()
	}
}
