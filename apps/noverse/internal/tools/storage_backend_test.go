package tools

import (
	"context"
	"encoding/json"
	"io/fs"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/ledger"
	"github.com/datnt1998/noverse/internal/store"
)

// memOps is an in-memory store.IOOps: it is the document backend's contract
// with no filesystem at all, so a test can prove a tool reads and writes only
// through the store facade.
type memOps struct {
	mu    sync.Mutex
	files map[string][]byte
}

func newMemOps() *memOps { return &memOps{files: map[string][]byte{}} }

func (m *memOps) Read(rel string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[rel]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: rel, Err: fs.ErrNotExist}
	}
	return append([]byte(nil), data...), nil
}

func (m *memOps) Write(rel string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[rel] = append([]byte(nil), data...)
	return nil
}

func (m *memOps) Append(rel string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[rel] = append(m.files[rel], data...)
	return nil
}

func (m *memOps) Sync(string) error { return nil }
func (m *memOps) Remove(rel string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, rel)
	return nil
}
func (m *memOps) RemoveTree(rel string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name := range m.files {
		if strings.HasPrefix(name, strings.TrimSuffix(rel, "/")+"/") {
			delete(m.files, name)
		}
	}
	return nil
}
func (m *memOps) Truncate(string, int64) error {
	return nil
}

func (m *memOps) ListDir(rel string) ([]store.DirEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix := strings.TrimSuffix(rel, "/") + "/"
	seen := map[string]bool{}
	var out []store.DirEntry
	for name := range m.files {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := strings.TrimPrefix(name, prefix)
		head := strings.SplitN(rest, "/", 2)[0]
		if head == "" || seen[head] {
			continue
		}
		seen[head] = true
		out = append(out, store.DirEntry{Name: head, IsDir: strings.Contains(rest, "/")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *memOps) Exists(rel string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.files[rel]
	return ok, nil
}
func (m *memOps) MkdirAll([]string) error { return nil }
func (m *memOps) BeginWrite()             {}
func (m *memOps) EndWrite() error         { return nil }

// memStore builds the facade over the in-memory document surface. Dir() is ""
// and there is no comment backend: exactly the hosted capability shape.
func memStore(t *testing.T) (*store.Store, *memOps) {
	t.Helper()
	ops := newMemOps()
	st, err := store.NewFromBackend(store.FileBackendOver(func() *store.IO {
		return store.NewIOFromOps(ops)
	}))
	if err != nil {
		t.Fatalf("store from in-memory backend: %v", err)
	}
	if st.Dir() != "" {
		t.Fatalf("in-memory backend must not expose a directory, got %q", st.Dir())
	}
	return st, ops
}

// TestEditChapterWritesThroughStore proves edit_chapter applies the mechanical
// prose replacement in memory and persists through DraftStore, with no
// workspace editor and no directory: the on-disk edit path is gone.
func TestEditChapterWritesThroughStore(t *testing.T) {
	st, ops := memStore(t)
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := st.Progress.Init(10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("UpdatePhase: %v", err)
	}
	if err := st.Drafts.SaveDraft(2, "Anh nắm chặt tay, đốt ngón trắng bệch."); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	if err := st.Progress.MarkChapterComplete(2, 40, "mystery", "quest"); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}
	if err := st.Progress.SetPendingRewrites([]int{2}, "test", store.RewriteCauseUser); err != nil {
		t.Fatalf("SetPendingRewrites: %v", err)
	}
	if err := st.Progress.SetFlow(domain.FlowPolishing); err != nil {
		t.Fatalf("SetFlow: %v", err)
	}

	tool := NewEditChapterTool(st)
	args, _ := json.Marshal(map[string]any{
		"chapter":    2,
		"old_string": "đốt ngón trắng bệch",
		"new_string": "khớp tay trắng bệch",
	})
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got, err := st.Drafts.LoadDraft(2)
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}
	if want := "Anh nắm chặt tay, khớp tay trắng bệch."; got != want {
		t.Fatalf("draft = %q, want %q", got, want)
	}
	ops.mu.Lock()
	_, stored := ops.files["drafts/02.draft.md"]
	ops.mu.Unlock()
	if !stored {
		t.Fatal("edit must persist the draft through the store document surface")
	}
}

// TestEditChapterKeepsMatchingSemantics pins the pre-existing behaviour the
// extraction must not change: quote/whitespace/NFC tolerance, ambiguity errors
// and the "identical replacement" rejection.
func TestEditChapterKeepsMatchingSemantics(t *testing.T) {
	cases := []struct {
		name string
		body string
		old  string
		new  string
		all  bool
		want string
		err  string
	}{
		{
			name: "typographic quotes and nbsp tolerate",
			body: "“Đi thôi,” cô nói.",
			old:  `"Đi thôi," cô nói.`,
			new:  "“Chạy đi,” cô nói.",
			want: "“Chạy đi,” cô nói.",
		},
		{
			name: "collapsed whitespace tolerates",
			body: "một   hai\nba",
			old:  "một hai ba",
			new:  "một hai bốn",
			want: "một hai bốn",
		},
		{
			name: "ambiguous without replace_all",
			body: "đi rồi đi",
			old:  "đi",
			new:  "chạy",
			err:  "mechanically equivalent occurrences",
		},
		{
			name: "identical replacement rejected",
			body: "một câu",
			old:  "một câu",
			new:  "một câu",
			err:  "giống new_string",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, _ := memStore(t)
			if err := st.Drafts.SaveDraft(1, tc.body); err != nil {
				t.Fatalf("SaveDraft: %v", err)
			}
			if err := st.Progress.Init(3); err != nil {
				t.Fatalf("Progress.Init: %v", err)
			}
			if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
				t.Fatalf("UpdatePhase: %v", err)
			}
			if err := st.Progress.MarkChapterComplete(1, 10, "", ""); err != nil {
				t.Fatalf("MarkChapterComplete: %v", err)
			}
			if err := st.Progress.SetPendingRewrites([]int{1}, "test", store.RewriteCauseUser); err != nil {
				t.Fatalf("SetPendingRewrites: %v", err)
			}
			if err := st.Progress.SetFlow(domain.FlowPolishing); err != nil {
				t.Fatalf("SetFlow: %v", err)
			}
			tool := NewEditChapterTool(st)
			args, _ := json.Marshal(map[string]any{
				"chapter": 1, "old_string": tc.old, "new_string": tc.new, "replace_all": tc.all,
			})
			_, err := tool.Execute(context.Background(), args)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("error = %v, want substring %q", err, tc.err)
				}
				// A rejected edit must leave the draft byte-identical.
				if got, _ := st.Drafts.LoadDraft(1); got != tc.body {
					t.Fatalf("draft changed on rejected edit: %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if got, _ := st.Drafts.LoadDraft(1); got != tc.want {
				t.Fatalf("draft = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestLedgerDocumentThroughStore proves the ledger no longer needs a directory:
// it round-trips through the store document surface.
func TestLedgerDocumentThroughStore(t *testing.T) {
	st, ops := memStore(t)
	rec := ledger.Record{Brief: "brief", Values: ledger.Values{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}}
	if err := ledger.SaveTo(st.Docs(), rec); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	ops.mu.Lock()
	_, stored := ops.files["meta/ledger.json"]
	ops.mu.Unlock()
	if !stored {
		t.Fatal("ledger must be stored as the meta/ledger.json document")
	}
	got, err := ledger.LoadFrom(st.Docs())
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got == nil || got.Brief != "brief" || got.Values != rec.Values {
		t.Fatalf("ledger round-trip = %+v", got)
	}
	// Missing document is (nil, nil), same as the file backend.
	if err := st.Docs().RemoveFile("meta/ledger.json"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if empty, err := ledger.LoadFrom(st.Docs()); err != nil || empty != nil {
		t.Fatalf("missing ledger = (%v, %v), want (nil, nil)", empty, err)
	}
}
