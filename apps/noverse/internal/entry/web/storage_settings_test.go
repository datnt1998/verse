package web

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestStorageSettingsFileModeContract(t *testing.T) {
	tg := newConfigTestGateway(t)
	res := tg.do(http.MethodGet, "/api/system/storage", tg.login("admin", "admin-pass"), "")
	var body struct {
		Mode       string `json:"mode"`
		Encryption struct {
			Available bool `json:"available"`
		} `json:"encryption"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if res.Code != 200 || body.Mode != "file" {
		t.Fatalf("storage mode = %d %q, want 200 file", res.Code, body.Mode)
	}
}
