package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"golang.org/x/text/unicode/norm"
)

func TestEditChapterMechanicalMatching(t *testing.T) {
	for _, tc := range []struct {
		name, draft, old, want string
		all, reject            bool
	}{
		{name: "NFC against NFD", draft: norm.NFD.String("Tiếng Việt"), old: "Tiếng Việt", want: "Đã sửa"},
		{name: "NFD against NFC", draft: "Tiếng Việt", old: norm.NFD.String("Tiếng Việt"), want: "Đã sửa"},
		{name: "curly quotes", draft: "“Chị An” nói ‘vâng’", old: "\"Chị An\" nói 'vâng'", want: "Đã sửa"},
		{name: "ellipsis", draft: "Chờ… nhé", old: "Chờ... nhé", want: "Đã sửa"},
		{name: "NBSP", draft: "Chị\u00a0An", old: "Chị An", want: "Đã sửa"},
		{name: "zero width", draft: "Tiếng\u200b Việt\u2060", old: "Tiếng Việt", want: "Đã sửa\u2060"},
		{name: "trailing spaces and blank lines", draft: "Đầu.  \n\n\nCuối.", old: "Đầu.\n\nCuối.", want: "Đã sửa"},
		{name: "trailing spaces in old string", draft: "Tiếng Việt", old: "Tiếng Việt  ", want: "Đã sửa"},
		{name: "untouched trailing spaces", draft: "Tiếng Việt  ", old: "Tiếng Việt", want: "Đã sửa  "},
		{name: "CRLF preserving untouched bytes", draft: "Trước\r\nĐầu.\r\nCuối.\r\nSau", old: "Đầu.\nCuối.", want: "Trước\r\nĐã sửa\r\nSau"},
		{name: "literal JSON newline", draft: "Đầu.\n\nCuối.", old: `Đầu.\n\nCuối.`, want: "Đã sửa"},
		{name: "preserve original byte spans", draft: "Trước " + norm.NFD.String("“Tiếng Việt…”") + " sau", old: "\"Tiếng Việt...\"", want: "Trước Đã sửa sau"},
		{name: "ambiguous normalization including exact", draft: "Chị An; Chị\u00a0An", old: "Chị An", reject: true},
		{name: "ambiguous quotes", draft: "\"An\"; “An”", old: "\"An\"", reject: true},
		{name: "ambiguous blank lines", draft: "Đầu.\nCuối.; Đầu.\n\nCuối.", old: "Đầu.\nCuối.", reject: true},
		{name: "replace all normalized variants", draft: "Chị An; Chị\u00a0An", old: "Chị An", all: true, want: "Đã sửa; Đã sửa"},
		{name: "paraphrase cannot use block anchors", draft: "Đầu.\nBản hiện tại đã thay đổi.\nCuối.", old: "Đầu.\nBản cũ đã thay đổi.\nCuối.", reject: true},
		{name: "stale read", draft: "Bà in trang cuối, nơi ghi rõ đây là dự thảo để xem xét.", old: "Bà in lại trang cuối, đặt trước mặt tôi.", reject: true},
		{name: "no candidate still returns current window", draft: "Một nội dung hoàn toàn mới.", old: "XYZ không tồn tại", reject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewStore(t.TempDir())
			if err := s.Init(); err != nil {
				t.Fatal(err)
			}
			if err := s.Progress.Init(10); err != nil {
				t.Fatal(err)
			}
			enterEditWritingPhase(t, s)
			queueCompletedChapterForEdit(t, s, 2, 10)
			if err := s.Drafts.SaveDraft(2, tc.draft); err != nil {
				t.Fatal(err)
			}
			args, _ := json.Marshal(map[string]any{"chapter": 2, "old_string": tc.old, "new_string": "Đã sửa", "replace_all": tc.all})
			_, err := NewEditChapterTool(s).Execute(context.Background(), args)
			got, loadErr := s.Drafts.LoadDraft(2)
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if tc.reject {
				if !errors.Is(err, errs.ErrToolPrecondition) {
					t.Fatalf("expected safe rejection, got %v", err)
				}
				if got != tc.draft {
					t.Fatalf("rejected edit changed original bytes: %q", got)
				}
				if !strings.HasPrefix(tc.name, "ambiguous") && (!strings.Contains(err.Error(), "Bản nháp hiện tại") || !strings.Contains(err.Error(), "read_chapter")) {
					t.Fatalf("missing current text/re-read guidance: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("mechanical mismatch should match: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
