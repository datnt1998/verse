package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// FindCutout returns the shared binding for an immutable original, if any.
func (s *CloneStore) FindCutout(ctx context.Context, hash string) (string, error) {
	var path string
	err := mediaDB(ctx, s.pool).QueryRow(ctx, `SELECT logical_path FROM clone_cutouts WHERE original_sha256=$1 LIMIT 1`, hash).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) { return "", nil }
	return path, err
}

// OwnCutout runs in the same media publication as the binding.
func (s *CloneStore) OwnCutout(ctx context.Context, job, hash, path string) error {
	db := mediaDB(ctx, s.pool)
	// Lock the job first, matching deletion and preventing a late owner from
	// appearing after deletion has collected its set of hashes.
	var id string
	if err := db.QueryRow(ctx, `SELECT id::text FROM clone_jobs WHERE id=$1 FOR KEY SHARE`, job).Scan(&id); err != nil { return err }
	if _, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 253026))`, hash); err != nil { return err }
	_, err := db.Exec(ctx, `INSERT INTO clone_cutouts(job_id,original_sha256,logical_path) VALUES($1,$2,$3) ON CONFLICT(job_id,original_sha256) DO NOTHING`, job, hash, path)
	return err
}

func (s *CloneStore) deleteCutouts(ctx context.Context, job string) error {
	db := mediaDB(ctx, s.pool)
	var id string
	err := db.QueryRow(ctx, `SELECT id::text FROM clone_jobs WHERE id=$1 FOR UPDATE`, job).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) { return nil }; if err != nil { return err }
	rows, err := db.Query(ctx, `SELECT original_sha256, logical_path FROM clone_cutouts WHERE job_id=$1 ORDER BY original_sha256`, job)
	if err != nil { return err }
	type cutout struct { hash, path string }
	var entries []cutout
	for rows.Next() { var c cutout; if err = rows.Scan(&c.hash, &c.path); err != nil { rows.Close(); return err }; entries = append(entries, c) }
	err = rows.Err(); rows.Close(); if err != nil { return err }
	if len(entries) == 0 { return nil }
	for _, c := range entries { if _, err = db.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 253026))`, c.hash); err != nil { return err } }
	if _, err = db.Exec(ctx, `DELETE FROM clone_cutouts WHERE job_id=$1`, job); err != nil { return err }
	state, err := readStorageState(ctx, db)
	if err != nil { return err }
	blobs := &BlobRepository{pool:s.pool}
	for _, c := range entries {
		var used bool
		if err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clone_cutouts WHERE logical_path=$1)`, c.path).Scan(&used); err != nil { return err }
		if !used { if err = blobs.Unbind(ctx, state.Active, c.path); err != nil { return err } }
	}
	return nil
}

// DeleteJobWithCutouts returns detached paths for local-file reclamation.
func (s *CloneStore) DeleteJobWithCutouts(ctx context.Context, job string) ([]string, error) {
	var paths []string
	err := s.inTx(ctx, func(tx context.Context) error {
		db := mediaDB(tx, s.pool)
		var id string
		if err := db.QueryRow(tx, `SELECT id::text FROM clone_jobs WHERE id=$1 FOR UPDATE`, job).Scan(&id); err != nil {
			return err
		}
		rows, err := db.Query(tx, `SELECT logical_path FROM clone_cutouts WHERE job_id=$1`, job)
		if err != nil { return err }
		for rows.Next() {
			var path string
			if err = rows.Scan(&path); err != nil { rows.Close(); return err }
			paths = append(paths, path)
		}
		err = rows.Err(); rows.Close()
		if err != nil { return err }
		return s.DeleteJob(tx, job)
	})
	return paths, err
}
