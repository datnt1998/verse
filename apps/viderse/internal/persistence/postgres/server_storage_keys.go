package postgres

import "context"

// SealedCredentials includes active, pending, previous and cancelled targets.
// A new key must not make any retained descriptor silently unreadable.
func (s *BlobRepository) SealedCredentials(ctx context.Context) ([][]byte, error) {
	rows, err := s.pool.Query(ctx, "SELECT credentials FROM storage_stores WHERE credentials IS NOT NULL ORDER BY created_at,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := [][]byte{}
	for rows.Next() {
		var value []byte
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}
