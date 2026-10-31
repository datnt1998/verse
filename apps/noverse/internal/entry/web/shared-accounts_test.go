package web

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSSOPublicSharedAccountsMode(t *testing.T) {
	f := newSSOFixture(t)
	w := authDo(t, f.g, http.MethodGet, "/api/setup", "", "")
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || body["sharedAccounts"] != true {
		t.Fatalf("public shared mode missing: %d %v", w.Code, body)
	}
}
