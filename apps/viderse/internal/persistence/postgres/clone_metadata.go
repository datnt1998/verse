package postgres

import (
	"context"
	"encoding/json"

	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/jackc/pgx/v5/pgtype"
)

// saveCloneMetadataState is called inside the same transaction as the artifact
// binding and checkpoint CAS, so history cannot diverge from the published file.
func (s *CloneStore) saveCloneMetadataState(ctx context.Context, jobID pgtype.UUID, state json.RawMessage) error {
	return s.q(ctx).SaveCloneMetadataState(ctx, gen.SaveCloneMetadataStateParams{JobID: jobID, State: state})
}
