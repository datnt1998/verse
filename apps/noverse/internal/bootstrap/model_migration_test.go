package bootstrap

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/core/llmconfig"
	"github.com/gofrs/flock"
)

// AU1: migration must be physical, not just hidden in /api/config. The stored
// project/story document loses provider/model pins while style, budget and
// every other non-model setting survives byte-for-byte in meaning.
func TestMigrateConfigFileKeepsNonModelSettings(t *testing.T) {
	writeGlobal(t, `{"provider":"openrouter","model":"m","providers":{"openrouter":{"api_key":"sk-test","models":[{"name":"m"}]}}}`)
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	story := filepath.Join(project, "stories", "01")
	if err := os.MkdirAll(filepath.Join(story, ".ainovel"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "project.json"), []byte(`{"name":"p"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	legacy := `{
  "provider": "xai-oauth",
  "model": "grok-4.7",
  "reasoning_effort": "low",
  "providers": {"xai-oauth": {"models": [{"name": "grok-4.7"}]}},
  "roles": {"writer": {"provider": "xai-oauth", "model": "grok-4.7"}},
  "style": "romance",
  "budget": {"book_usd": 12.5, "warn_ratio": 0.8},
  "advance_mode": "review",
  "tts": {"engine": "openai-speech", "quality": "high"},
  "future_setting": {"keep": [1, true, "unknown"]}
}`
	path := filepath.Join(story, ".ainovel", "config.json")
	if err := os.WriteFile(path, []byte(legacy), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadConfigChain(story); err != nil {
		t.Fatalf("LoadConfigChain: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]any
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("stored document is not JSON: %v\n%s", err, data)
	}
	for _, key := range modelSelectionKeys {
		if _, ok := stored[key]; ok {
			t.Fatalf("stored document still pins %q: %s", key, data)
		}
	}
	if stored["style"] != "romance" || stored["advance_mode"] != "review" {
		t.Fatalf("non-model settings lost: %+v", stored)
	}
	budget, _ := stored["budget"].(map[string]any)
	if budget["book_usd"] != 12.5 || budget["warn_ratio"] != 0.8 {
		t.Fatalf("budget lost: %+v", stored["budget"])
	}
	tts, _ := stored["tts"].(map[string]any)
	if tts["engine"] != "openai-speech" || tts["quality"] != "high" {
		t.Fatalf("tts lost: %+v", stored["tts"])
	}
	if got := stored["future_setting"].(map[string]any)["keep"].([]any); len(got) != 3 || got[2] != "unknown" {
		t.Fatalf("unknown raw keys lost: %+v", stored)
	}

	// Idempotent: a second load does not rewrite the file again.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("migration changed mode to %o", info.Mode().Perm())
	}
	if _, _, err := LoadConfigChain(story); err != nil {
		t.Fatal(err)
	}
	again, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !again.ModTime().Equal(info.ModTime()) {
		t.Fatal("second load rewrote an already-migrated document")
	}
}

func TestMigrateConfigDocumentStripsOnlyModelKeys(t *testing.T) {
	raw := []byte(`{
  "provider": "xai-oauth",
  "model": "grok-4.7",
  "roles": {"writer": {"provider": "xai-oauth"}},
  "providers": {"xai-oauth": {}},
  "reasoning_effort": "low",
  "style": "xianxia",
  "notify": {"enabled": true, "events": ["done"]}
}`)
	out, changed, err := MigrateConfigDocument(raw)
	if err != nil || !changed {
		t.Fatalf("MigrateConfigDocument: changed=%v err=%v", changed, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range modelSelectionKeys {
		if _, ok := doc[key]; ok {
			t.Fatalf("still has %q: %s", key, out)
		}
	}
	if doc["style"] != "xianxia" {
		t.Fatalf("style lost: %s", out)
	}
	notify, _ := doc["notify"].(map[string]any)
	if notify["enabled"] != true {
		t.Fatalf("notify lost: %s", out)
	}
	if _, changed, err := MigrateConfigDocument(out); err != nil || changed {
		t.Fatalf("already-clean document changed=%v err=%v", changed, err)
	}
}

// Run the real writers in another process while an independent flock holder
// controls the transaction boundary. The readiness handshake means migration
// starts before writer B commits, not after an accidentally favourable schedule.
func TestConfigFileCrossProcessWriters(t *testing.T) {
	if action := os.Getenv("AINOVEL_CONFIG_WRITER_TEST"); action != "" {
		path := os.Getenv("AINOVEL_CONFIG_WRITER_PATH")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		// The parent holds the lock, so this pre-write fixture is stable.
		snapshot := ConfigFileSnapshot{Data: data, Info: info}
		fmt.Println("ready")
		switch action {
		case "migration":
			_, err = MigrateConfigFile(path)
		case "save":
			err = SaveConfig(path, Config{Style: "writer"})
		case "provider":
			err = SaveProviderConfig(path, "ollama", llmconfig.ProviderConfig{Models: []llmconfig.ModelConfig{{Name: "m"}}})
		case "cas":
			err = SaveConfigIfUnchanged(path, Config{Style: "writer"}, snapshot)
			if errors.Is(err, errs.ErrStorageConflict) {
				err = nil // writer B legitimately won after this child's snapshot
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Setenv("HOME", t.TempDir())
	for _, scope := range []string{"global", "project", "story"} {
		for _, action := range []string{"migration", "save", "provider", "cas"} {
			t.Run(scope+"/"+action, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), scope, ".ainovel", "config.json")
				if err := SaveConfig(path, Config{Provider: "old", ModelName: "old-model", Style: "old"}); err != nil {
					t.Fatal(err)
				}
				lock := flock.New(path + ".lock")
				if err := lock.Lock(); err != nil {
					t.Fatal(err)
				}
				defer lock.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestConfigFileCrossProcessWriters$")
				cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), "AINOVEL_CONFIG_WRITER_TEST="+action, "AINOVEL_CONFIG_WRITER_PATH="+path)
				stdout, err := cmd.StdoutPipe()
				if err != nil {
					t.Fatal(err)
				}
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				scanner := bufio.NewScanner(stdout)
				if !scanner.Scan() || scanner.Text() != "ready" {
					t.Fatalf("child did not reach write boundary: %s %s", scanner.Text(), stderr.String())
				}
				done := make(chan error, 1)
				go func() { done <- cmd.Wait() }()
				select {
				case err := <-done:
					t.Fatalf("%s bypassed the shared cross-process lock: %v %s", action, err, stderr.String())
				case <-time.After(150 * time.Millisecond):
				}
				// Writer B's exact locked persistence path: migration must read
				// this new style/budget after the lock is released, not its old A.
				cfg := Config{Provider: "old", ModelName: "old-model", Style: "new"}
				cfg.Budget.BookUSD = 23.5
				if err := saveConfigLocked(path, cfg); err != nil {
					t.Fatal(err)
				}
				if err := lock.Unlock(); err != nil {
					t.Fatal(err)
				}
				if err := <-done; err != nil {
					t.Fatalf("child writer: %v %s", err, stderr.String())
				}
				if action == "migration" || action == "provider" || action == "cas" {
					got, err := LoadConfigFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if got.Style != "new" || got.Budget.BookUSD != 23.5 {
						t.Fatalf("%s lost writer B's non-model settings: %+v", action, got)
					}
					if action == "migration" && got.Provider != "" {
						t.Fatal("migration left the old model selection")
					}
				}
			})
		}
	}
}

func TestConfigSnapshotRejectsStaleWrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "config.json")
	first := Config{Style: "first"}
	if err := SaveConfig(path, first); err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReadConfigFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	second := Config{Style: "second"}
	second.Budget.BookUSD = 41
	if err := SaveConfig(path, second); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfigIfUnchanged(path, first, snapshot); !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("stale snapshot must conflict, got %v", err)
	}
	got, err := LoadConfigFile(path)
	if err != nil || got.Style != "second" || got.Budget.BookUSD != 41 {
		t.Fatalf("stale writer overwrote current budget/style: %+v %v", got, err)
	}
}

func TestMigrateConfigFileLeavesDamagedDocumentUntouched(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "config.json")
	damaged := []byte(`{"provider":"old","budget":`)
	if err := os.WriteFile(path, damaged, 0o640); err != nil {
		t.Fatal(err)
	}
	if changed, err := MigrateConfigFile(path); changed || err == nil {
		t.Fatalf("damaged migration: changed=%v err=%v", changed, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, damaged) {
		t.Fatalf("damaged document was overwritten: %s %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("damaged file mode changed: %v %v", info, err)
	}
}

func TestConfigSnapshotRejectsIdenticalReplacement(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Config{Style: "unchanged"}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReadConfigFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfigIfUnchanged(path, cfg, snapshot); !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("identical replacement did not invalidate snapshot: %v", err)
	}
}
