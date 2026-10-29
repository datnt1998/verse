package tools

import (
	"context"
	"fmt"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/ledger"
)

func (t *SaveFoundationTool) checkLedger(ctx context.Context, kind, content string, axes []string) (map[string]any, error) {
	// Sibling-project history is a file-library feature: the ledger compares
	// stories inside one project directory. Hosted stores have no project
	// directory (and must not scan one), so there is no prior set to compare.
	var prior []ledger.Story
	if dir, ok := t.store.FileRoot(); ok {
		var err error
		prior, err = ledger.ProjectHistory(dir)
		if err != nil {
			return nil, fmt.Errorf("Đọc sổ cái: %w: %w", errs.ErrStoreRead, err)
		}
	}
	rec, err := ledger.LoadFrom(t.store.Docs())
	if err != nil {
		return nil, err
	}
	if kind == "premise" {
		if axes != nil && len(axes) != 10 {
			return nil, fmt.Errorf("axes cần đúng 10 giá trị: %w", errs.ErrToolArgs)
		}
		var v ledger.Values
		if axes != nil {
			copy(v[:], axes)
		} else if t.ledgerModel != nil && fileLibrary(t.store) {
			p, err := ledger.Analyze(ctx, t.ledgerModel, content, prior, false)
			if err != nil {
				return nil, err
			}
			v = p.Values
		} else if len(prior) == 0 {
			return nil, nil
		} else {
			return nil, fmt.Errorf("Cần mười trục trước khi lưu brief: %w", errs.ErrToolArgs)
		}
		if err := ledger.Validate(v); err != nil {
			return nil, fmt.Errorf("%w: %w", err, errs.ErrToolArgs)
		}
		rec = &ledger.Record{Values: v, Brief: content, Result: ledger.CheckValues(v, prior)}
		if err := ledger.SaveTo(t.store.Docs(), *rec); err != nil {
			return nil, err
		}
	} else if rec != nil {
		rec.Result = ledger.CheckValues(rec.Values, prior)
		if err := ledger.SaveTo(t.store.Docs(), *rec); err != nil {
			return nil, err
		}
	} else if len(prior) > 0 {
		return nil, fmt.Errorf("Lưu brief kèm mười trục trước khi lên đại cương: %w", errs.ErrToolPrecondition)
	}
	if rec != nil && rec.Result.Blocking() {
		if kind == "premise" {
			if err := t.store.Outline.SavePremise(content); err != nil {
				return nil, err
			}
		}
		items := make([]map[string]any, 0, len(rec.Result.Checks))
		for _, check := range rec.Result.Checks {
			if check.Level == "ok" {
				continue
			}
			items = append(items, map[string]any{
				"id": "ledger-" + check.Rule, "level": check.Level,
				"ref": "Hệ thống sản xuất liên tục §1", "refName": "Luật khoảng cách",
				"text": check.Text, "quote": "", "fix": "Đổi trục trong brief trước khi lên đại cương.",
			})
		}
		return map[string]any{"saved": kind == "premise", "type": kind, "foundation_ready": false, "blocking": true, "checks": rec.Result.Checks, "items": items, "message": "Truyện này giống một truyện bạn đã đăng. Đổi trục trong brief trước khi lên đại cương."}, nil
	}
	return nil, nil
}
