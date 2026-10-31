package blobstore

// CopySourceError preserves error identity so an operator can distinguish a
// missing source object from a missing destination bucket without inspecting
// provider messages or duplicating the verification protocol.
type CopySourceError struct{ Err error }

func (e *CopySourceError) Error() string { return e.Err.Error() }
func (e *CopySourceError) Unwrap() error { return e.Err }
