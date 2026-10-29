package blobstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// CopyReport summarizes one copy run. The caller exits nonzero on any
// mismatch/conflict/error and never moves a live pointer until
// Mismatches+Conflicts+Errors == 0.
type CopyReport struct {
	SourceObjects int
	Copied        int
	Matched       int // already at target with identical sha256+size
	Conflicts     int // target exists with different content
	Mismatches    int // written object failed download verification
	Errors        int
}

// LedgerEntry is one line of the manifest ledger. Immutable keys make it
// resumable: a recorded copied/matched key is skipped on --resume.
type LedgerEntry struct {
	Key    string `json:"key"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Status string `json:"status"` // copied|matched|conflict|mismatch|error
	Detail string `json:"detail,omitempty"`
	At     string `json:"at"`
}

// CopyOptions controls Copier.
type CopyOptions struct {
	// ManifestPath is the JSONL ledger. Created mode 0600.
	ManifestPath string
	// Resume continues from an existing manifest. Without Resume a non-empty
	// manifest is an error, so a stale ledger can never skip silently.
	Resume bool
	// Concurrency bounds parallel objects; <=0 runs sequentially.
	Concurrency int
	// Progress receives each ledger entry after it is persisted; may be nil.
	Progress func(LedgerEntry)
}

// Copier moves blobs between two stores with full download verification.
type Copier struct {
	src BlobStore
	dst BlobStore
}

// NewCopier builds a copier. Direction local↔s3 and s3↔s3 are all supported —
// only the BlobStore contract is used.
func NewCopier(src, dst BlobStore) *Copier {
	return &Copier{src: src, dst: dst}
}

// Run lists the whole source (paginated, streamed), copies absent objects and
// verifies every touched object by re-downloading the target and comparing
// SHA-256 + size. Same target hash → skip (matched); different hash →
// conflict, never overwritten. Source is never modified or deleted.
func (c *Copier) Run(ctx context.Context, opts CopyOptions) (CopyReport, error) {
	var rep CopyReport
	done := map[string]LedgerEntry{}
	var ledger *os.File
	if opts.ManifestPath != "" {
		if err := loadLedger(opts.ManifestPath, opts.Resume, done); err != nil {
			return rep, err
		}
		f, err := os.OpenFile(opts.ManifestPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return rep, fmt.Errorf("mở manifest: %w", err)
		}
		if err := f.Chmod(0o600); err != nil {
			f.Close()
			return rep, fmt.Errorf("quyền manifest: %w", err)
		}
		ledger = f
		defer ledger.Close()
	}
	workers := opts.Concurrency
	if workers <= 0 {
		workers = 1
	}

	var mu sync.Mutex
	var firstErr error
	tally := func(status string) {
		mu.Lock()
		defer mu.Unlock()
		switch status {
		case "copied":
			rep.Copied++
		case "matched":
			rep.Matched++
		case "conflict":
			rep.Conflicts++
		case "mismatch":
			rep.Mismatches++
		default:
			rep.Errors++
		}
	}
	record := func(e LedgerEntry) {
		e.At = time.Now().UTC().Format(time.RFC3339)
		mu.Lock()
		if ledger != nil {
			b, _ := json.Marshal(e)
			_, _ = ledger.Write(append(b, '\n'))
			_ = ledger.Sync()
		}
		if opts.Progress != nil {
			opts.Progress(e)
		}
		mu.Unlock()
		tally(e.Status)
	}

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	cursor := ""
	for {
		page, err := c.src.List(ctx, "", cursor, 1000)
		if err != nil {
			return rep, fmt.Errorf("liệt kê nguồn: %w", err)
		}
		for _, info := range page.Infos {
			mu.Lock()
			rep.SourceObjects++
			mu.Unlock()
			if prev, ok := done[info.Key]; ok && (prev.Status == "copied" || prev.Status == "matched") {
				// Resume: ledger says this immutable key was verified — but a
				// Stat is cheap, so confirm the object still exists rather
				// than trusting the ledger blindly.
				if _, err := c.dst.Stat(ctx, info.Key); err == nil {
					tally(prev.Status)
					continue
				} else if !IsNotFound(err) {
					record(LedgerEntry{Key: info.Key, Status: "error", Detail: "stat đích khi resume: " + err.Error()})
					continue
				}
			}
			key := info.Key
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				entry, err := c.copyOne(ctx, key)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					if entry.Key == "" {
						entry.Key = key
					}
					entry.Status = "error"
					entry.Detail = err.Error()
				}
				record(entry)
			}()
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	wg.Wait()
	return rep, firstErr
}

// copyOne transfers one immutable key. It returns the ledger entry; a non-nil
// error means the entry could not be classified (transport/API failure) and
// the key must be retried — it is recorded as an error, never as copied.
func (c *Copier) copyOne(ctx context.Context, key string) (LedgerEntry, error) {
	_, dstErr := c.dst.Stat(ctx, key)
	switch {
	case dstErr == nil:
		// Target exists: compare, never overwrite.
		srcSHA, srcSize, err := hashOf(ctx, c.src, key)
		if err != nil {
			return LedgerEntry{Key: key}, fmt.Errorf("đọc nguồn để đối soát: %w", err)
		}
		dstSHA, dstSize, err := hashOf(ctx, c.dst, key)
		if err != nil {
			return LedgerEntry{Key: key}, fmt.Errorf("đọc đích để đối soát: %w", err)
		}
		if srcSHA == dstSHA && srcSize == dstSize {
			return LedgerEntry{Key: key, Size: srcSize, SHA256: srcSHA, Status: "matched"}, nil
		}
		return LedgerEntry{
			Key: key, Size: dstSize, SHA256: dstSHA, Status: "conflict",
			Detail: fmt.Sprintf("đích khác nội dung (nguồn %s/%d)", srcSHA, srcSize),
		}, nil
	case IsNotFound(dstErr):
		// fresh copy below
	default:
		return LedgerEntry{Key: key}, fmt.Errorf("stat đích: %w", dstErr)
	}

	rc, srcHead, err := c.src.Open(ctx, key, FullRange)
	if err != nil {
		return LedgerEntry{Key: key}, fmt.Errorf("mở nguồn: %w", err)
	}
	putInfo, err := c.dst.Put(ctx, key, rc, PutOptions{ContentType: srcHead.ContentType})
	rc.Close()
	if err != nil {
		return LedgerEntry{Key: key}, fmt.Errorf("ghi đích: %w", err)
	}
	// Verify by re-downloading the target: ETag is never trusted as a digest.
	dstSHA, dstSize, err := hashOf(ctx, c.dst, key)
	if err != nil {
		return LedgerEntry{Key: key}, fmt.Errorf("đọc lại đích: %w", err)
	}
	if dstSHA != putInfo.SHA256 || dstSize != putInfo.Size {
		return LedgerEntry{
			Key: key, Size: dstSize, SHA256: dstSHA, Status: "mismatch",
			Detail: fmt.Sprintf("ghi %s/%d nhưng đọc lại %s/%d", putInfo.SHA256, putInfo.Size, dstSHA, dstSize),
		}, nil
	}
	return LedgerEntry{Key: key, Size: dstSize, SHA256: dstSHA, Status: "copied"}, nil
}

// hashOf streams a blob and returns its canonical SHA-256 and size.
func hashOf(ctx context.Context, s BlobStore, key string) (string, int64, error) {
	rc, _, err := s.Open(ctx, key, FullRange)
	if err != nil {
		return "", 0, err
	}
	defer rc.Close()
	return DigestAll(rc)
}

// loadLedger reads an existing manifest. Without resume a non-empty manifest
// is an error: re-running without --resume must not silently skip keys based
// on a stale ledger.
func loadLedger(path string, resume bool, done map[string]LedgerEntry) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("đọc manifest: %w", err)
	}
	if len(bytes.TrimSpace(data)) > 0 && !resume {
		return fmt.Errorf("manifest đã tồn tại; dùng --resume để tiếp tục hoặc chỉ định manifest khác")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var e LedgerEntry
		if err := dec.Decode(&e); err != nil {
			return fmt.Errorf("manifest lỗi dòng: %w", err)
		}
		done[e.Key] = e
	}
	return nil
}
