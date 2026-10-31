package identity

// RecordSystemAudit records an idempotent administrative maintenance action.
// The (action, target) pair identifies the operation across interrupted retries.
// Repository-backed stores must explicitly support the capability.
func (s *Store) RecordSystemAudit(action, target, reason string) error {
	if s.repo != nil {
		if recorder, ok := s.repo.(interface {
			RecordSystemAudit(string, string, string) error
		}); ok {
			return recorder.RecordSystemAudit(action, target, reason)
		}
		return ErrUnsupported
	}
	return s.update(func(d *data) error {
		for _, entry := range d.Audit {
			if entry.Actor == nil && entry.Action == action && entry.Target == target {
				return nil
			}
		}
		s.addAudit(d, nil, "perm", action, target, "", "", reason)
		return nil
	})
}
