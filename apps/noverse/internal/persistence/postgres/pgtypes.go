package postgres

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/platform/identity"
)

// Small pgtype/time helpers shared by the Noverse domain repositories. The
// account repository moved to platform/identity/postgres and keeps its own
// copies; only helpers that non-identity code really uses live here.

// deterministicUUID pins singleton documents and project audio metadata to the
// same stable namespace used by the imported hosted storage format.
func deterministicUUID(namespace, name string) pgtype.UUID {
	sum := sha256.Sum256([]byte("ainovel|v1|" + namespace + "|" + name))
	var id pgtype.UUID
	copy(id.Bytes[:], sum[:16])
	id.Bytes[6] = id.Bytes[6]&0x0f | 0x50
	id.Bytes[8] = id.Bytes[8]&0x3f | 0x80
	id.Valid = true
	return id
}

func randomUUID() (pgtype.UUID, error) {
	var id pgtype.UUID
	if _, err := rand.Read(id.Bytes[:]); err != nil {
		return id, err
	}
	id.Bytes[6] = id.Bytes[6]&0x0f | 0x40
	id.Bytes[8] = id.Bytes[8]&0x3f | 0x80
	id.Valid = true
	return id, nil
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }

func tsPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}

func noRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// constraintErr maps Postgres unique violations to the repository contract's
// domain errors so callers never see raw SQLSTATEs.
func constraintErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch {
		case strings.Contains(pgErr.ConstraintName, "project_one_owner"):
			return identity.ErrOwnerTaken
		case strings.Contains(pgErr.ConstraintName, "username") || strings.Contains(pgErr.ConstraintName, "users"):
			return identity.ErrExists
		default:
			return fmt.Errorf("ràng buộc duy nhất %s: %w", pgErr.ConstraintName, errs.ErrStorageConflict)
		}
	}
	return sanitizeErr(err)
}
