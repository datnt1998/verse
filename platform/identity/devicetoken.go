package identity

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

// DeviceTokenTTL is how long a browser stays a "known device" of one account.
// The token never authenticates anything by itself: it only tells the login
// limiter that this device already proved the account password, so a failure
// aimed at the account from elsewhere cannot lock it out.
const DeviceTokenTTL = 180 * 24 * time.Hour

// deviceTokenBytes is the entropy of one token (64 hex characters). A token
// that does not have exactly this shape never reaches storage.
const deviceTokenBytes = 32

// DeviceToken is a freshly minted device credential. Token is the only copy of
// the plaintext — it goes to the browser and never to storage; ExpiresAt is the
// server-side lifetime the cookie must not outlive.
type DeviceToken struct {
	Token     string
	ExpiresAt time.Time
}

// NewDeviceToken mints a random token expiring at expiresAt.
func NewDeviceToken(expiresAt time.Time) (DeviceToken, error) {
	b := make([]byte, deviceTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return DeviceToken{}, err
	}
	return DeviceToken{Token: hex.EncodeToString(b), ExpiresAt: expiresAt}, nil
}

// DeviceTokenHash is the stored form of a device token: only SHA-256 is kept,
// exactly like session secrets, so a database copy cannot be replayed.
func DeviceTokenHash(token string) string { return digest(token) }

// ValidDeviceTokenShape rejects anything that was not minted here before it can
// reach storage or a limiter bucket key.
func ValidDeviceTokenShape(token string) bool {
	if len(token) != deviceTokenBytes*2 {
		return false
	}
	for _, c := range token {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// DeviceBucket is the login-limiter bucket of one (account, device) pair: a
// known device is metered on its own budget instead of the shared account
// bucket. The suffix is a truncated digest, never the token itself.
func DeviceBucket(username, token string) string {
	h := DeviceTokenHash(token)
	return "dev:" + strings.ToLower(username) + ":" + h[:16]
}

// IssueDeviceToken mints a token for username and stores its digest. Only a
// successful credential check may call it.
func (s *Store) IssueDeviceToken(username string) (DeviceToken, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if !ValidUsername(username) {
		return DeviceToken{}, ErrInvalidInput
	}
	token, err := NewDeviceToken(s.clock().Add(DeviceTokenTTL))
	if err != nil {
		return DeviceToken{}, err
	}
	if err := s.SaveDeviceToken(username, DeviceTokenHash(token.Token), token.ExpiresAt); err != nil {
		return DeviceToken{}, err
	}
	return token, nil
}

// LoginBuckets returns the limiter buckets charged by one login attempt on
// username from ip, with the device token the browser presented ("" when the
// cookie is absent). keys[0] is always the per-IP bucket; keys[1], when present,
// is the account-scoped bucket: the shared "user:<name>" bucket for an unknown
// device, or a per-(account, device) bucket when the token is a live device
// token of that account. device reports whether the token was accepted, so the
// caller can keep the cookie instead of minting a new one.
//
// A forged, expired or foreign token is an unknown device: it falls back to the
// shared account bucket, never to a bucket of its own.
func (s *Store) LoginBuckets(username, ip, deviceToken string) (keys []string, device bool) {
	keys = []string{"ip:" + ip}
	if !ValidUsername(username) {
		return keys, false
	}
	token := strings.TrimSpace(deviceToken)
	if token != "" && ValidDeviceTokenShape(token) {
		if ok, err := s.DeviceTokenValid(username, DeviceTokenHash(token)); err == nil && ok {
			return append(keys, DeviceBucket(username, token)), true
		}
	}
	return append(keys, "user:"+username), false
}

// SaveDeviceToken records the digest of a device token for username.
func (s *Store) SaveDeviceToken(username, tokenHash string, expiresAt time.Time) error {
	if s.repo != nil {
		return s.repo.SaveDeviceToken(username, tokenHash, expiresAt)
	}
	username = strings.ToLower(strings.TrimSpace(username))
	return s.update(func(d *data) error {
		if d.user(username) == nil {
			return ErrNotFound
		}
		now := s.clock()
		d.pruneDeviceTokens(now)
		d.DeviceTokens = append(d.DeviceTokens, deviceToken{
			Username: username, Hash: tokenHash, CreatedAt: now, ExpiresAt: expiresAt,
		})
		d.capDeviceTokens(username, maxDeviceTokens)
		return nil
	})
}

// DeviceTokenValid reports whether tokenHash is a live device token of
// username. Unknown accounts answer false, never ErrNotFound: the caller must
// not learn whether the account exists from a limiter hint.
func (s *Store) DeviceTokenValid(username, tokenHash string) (bool, error) {
	if s.repo != nil {
		return s.repo.DeviceTokenValid(username, tokenHash)
	}
	username = strings.ToLower(strings.TrimSpace(username))
	d, err := s.snapshot()
	if err != nil {
		return false, err
	}
	if d.user(username) == nil {
		return false, nil
	}
	now := s.clock()
	for _, t := range d.DeviceTokens {
		if t.Username == username && t.ExpiresAt.After(now) && subtleCompare(t.Hash, tokenHash) {
			return true, nil
		}
	}
	return false, nil
}

// RevokeDeviceTokens forgets every known device of the account. Password
// change, recovery, disable, delete and revoke-all all call it: a device that
// was trusted under the old credential must earn it again.
func (s *Store) RevokeDeviceTokens(username string) error {
	if s.repo != nil {
		return s.repo.RevokeDeviceTokens(username)
	}
	username = strings.ToLower(strings.TrimSpace(username))
	return s.update(func(d *data) error {
		d.dropDeviceTokens(username)
		return nil
	})
}
