package pgtest

import (
	"strings"
	"testing"
)

func TestSchemaNameUniqueAndBounded(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		n := SchemaName(t)
		if len(n) > 63 {
			t.Fatalf("tên %q dài %d > 63", n, len(n))
		}
		if !strings.HasPrefix(n, "pgtest_") {
			t.Fatalf("thiếu tiền tố: %q", n)
		}
		if seen[n] {
			t.Fatalf("trùng tên: %q", n)
		}
		seen[n] = true
	}
}
