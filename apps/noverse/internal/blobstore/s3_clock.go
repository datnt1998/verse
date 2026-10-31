package blobstore

import (
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/middleware"
	"time"
)

func (s *S3) recordClock(metadata middleware.Metadata) {
	server, ok := awsmiddleware.GetServerTime(metadata)
	if !ok {
		return
	}
	received, ok := awsmiddleware.GetResponseAt(metadata)
	if !ok {
		return
	}
	s.clockOffset.Store(int64(server.Sub(received)))
	s.clockKnown.Store(true)
}

// ServerClockOffset compares the provider's Date response header with the
// application's receipt clock. It does not rely on a provider rejecting future
// SigV4 timestamps (Garage accepts them), and performs no extra network call.
func (s *S3) ServerClockOffset() (time.Duration, bool) {
	return time.Duration(s.clockOffset.Load()), s.clockKnown.Load()
}
