package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/aiconn"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
)

// AIConnStore là store của mảng kết nối AI trên ai_providers + ai_models —
// hiện thực aiconn.Store. Dữ liệu nhạy cảm (key_enc) qua nguyên bytea, không
// bao giờ bị ghi log.
type AIConnStore struct {
	pool *pgxpool.Pool // nil khi dựng trên DBTX không phải pool
	q    *gen.Queries
}

// NewAIConnStore dựng store trên pool (transaction cần cho ReplaceCatalog).
func NewAIConnStore(pool *pgxpool.Pool) *AIConnStore {
	return &AIConnStore{pool: pool, q: gen.New(pool)}
}

// NewAIConnStoreOn dựng store trên một DBTX bất kỳ (test transaction);
// ReplaceCatalog cần DBTX là pool hoặc *pgx.Tx — các DBTX khác trả lỗi.
func NewAIConnStoreOn(db gen.DBTX) *AIConnStore {
	if pool, ok := db.(*pgxpool.Pool); ok {
		return NewAIConnStore(pool)
	}
	return &AIConnStore{q: gen.New(db)}
}

// providerToDomain đổi dòng gen sang kiểu domain (selected jsonb → []string).
func providerToDomain(r gen.AiProvider) domain.Provider {
	var selected []string
	if err := json.Unmarshal(r.Selected, &selected); err != nil || selected == nil {
		selected = []string{}
	}
	p := domain.Provider{
		ID:       r.ID,
		Kind:     r.Kind,
		Auth:     r.Auth,
		Label:    r.Label,
		EncKey:   r.KeyEnc,
		KeyHint:  r.KeyHint,
		BaseURL:  r.BaseUrl,
		Status:   r.Status,
		Selected: selected,
	}
	if r.CatalogAt.Valid {
		p.CatalogAt = r.CatalogAt.Time.UnixMilli()
	}
	if r.CreatedAt.Valid {
		p.CreatedAt = r.CreatedAt.Time.UnixMilli()
	}
	if r.UpdatedAt.Valid {
		p.UpdatedAt = r.UpdatedAt.Time.UnixMilli()
	}
	return p
}

// modelToDomain đổi dòng gen sang kiểu domain.
func modelToDomain(r gen.AiModel) domain.AIModel {
	var caps []string
	if err := json.Unmarshal(r.Caps, &caps); err != nil {
		caps = []string{}
	}
	m := domain.AIModel{
		ID:          r.ID,
		Recommended: r.Recommended,
		Manual:      r.Manual,
	}
	for _, c := range caps {
		m.Caps = append(m.Caps, c)
	}
	if r.ContextK.Valid {
		m.ContextK = int(r.ContextK.Int32)
	}
	if r.InPerM.Valid {
		m.InPerM = r.InPerM.Float64
	}
	if r.OutPerM.Valid {
		m.OutPerM = r.OutPerM.Float64
	}
	if r.PerImage.Valid {
		m.PerImage = r.PerImage.Float64
	}
	return m
}

// ListProviders trả mọi thẻ kèm số model trong một truy vấn (không N+1).
func (s *AIConnStore) ListProviders(ctx context.Context) ([]aiconn.ProviderSummary, error) {
	rows, err := s.q.ListAiProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("đọc ai_providers: %w", err)
	}
	out := make([]aiconn.ProviderSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, aiconn.ProviderSummary{
			Provider: providerToDomain(gen.AiProvider{
				ID: r.ID, Kind: r.Kind, Auth: r.Auth, Label: r.Label,
				KeyEnc: r.KeyEnc, KeyHint: r.KeyHint, BaseUrl: r.BaseUrl,
				Status: r.Status, CatalogAt: r.CatalogAt, Selected: r.Selected,
				CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			}),
			Count: int(r.ModelCount),
		})
	}
	return out, nil
}

// ListSubscribed trả các thẻ đăng nhập gói + số model (đường đọc attention).
func (s *AIConnStore) ListSubscribed(ctx context.Context) ([]aiconn.ProviderSummary, error) {
	rows, err := s.q.ListAiSubscribed(ctx)
	if err != nil {
		return nil, fmt.Errorf("đọc ai_providers (subscription): %w", err)
	}
	out := make([]aiconn.ProviderSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, aiconn.ProviderSummary{
			Provider: providerToDomain(gen.AiProvider{
				ID: r.ID, Kind: r.Kind, Auth: r.Auth, Label: r.Label,
				KeyEnc: r.KeyEnc, KeyHint: r.KeyHint, BaseUrl: r.BaseUrl,
				Status: r.Status, CatalogAt: r.CatalogAt, Selected: r.Selected,
				CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			}),
			Count: int(r.ModelCount),
		})
	}
	return out, nil
}

// GetProvider trả một thẻ; vắng → Provider{} không lỗi.
func (s *AIConnStore) GetProvider(ctx context.Context, id string) (domain.Provider, error) {
	r, err := s.q.GetAiProvider(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Provider{}, nil
	}
	if err != nil {
		return domain.Provider{}, fmt.Errorf("đọc ai_providers.%s: %w", id, err)
	}
	return providerToDomain(r), nil
}

// UpsertProvider thêm một dòng provider mới. Mọi thay đổi sau đó đi qua các
// câu ghi từng trường (SetSelected / UpdateProviderKey / SetStatus) để hai tab
// sửa hai trường khác nhau không ghi đè lẫn nhau.
func (s *AIConnStore) UpsertProvider(ctx context.Context, p domain.Provider) error {
	selected, err := json.Marshal(p.Selected)
	if err != nil {
		return err
	}
	if p.Status == "" {
		p.Status = domain.StatusReady
	}
	if p.Selected == nil {
		selected, _ = json.Marshal([]string{})
	}
	if _, err := s.q.InsertAiProvider(ctx, gen.InsertAiProviderParams{
		ID: p.ID, Kind: p.Kind, Auth: p.Auth, Label: p.Label,
		KeyEnc: p.EncKey, KeyHint: p.KeyHint, BaseUrl: p.BaseURL,
		Status: p.Status, Selected: selected,
	}); err != nil {
		return fmt.Errorf("ghi ai_providers.%s: %w", p.ID, err)
	}
	return nil
}

// UpdateProviderKey chỉ đổi khoá/địa chỉ/trạng thái — lựa chọn model do
// SetSelected quản nên PATCH khoá không ghi đè lựa chọn mới.
func (s *AIConnStore) UpdateProviderKey(ctx context.Context, id string, encKey []byte, keyHint, baseURL string, status domain.AIStatus) error {
	if status == "" {
		status = domain.StatusReady
	}
	if err := s.q.UpdateAiProviderKey(ctx, gen.UpdateAiProviderKeyParams{
		ID: id, KeyEnc: encKey, KeyHint: keyHint, BaseUrl: baseURL, Status: status,
	}); err != nil {
		return fmt.Errorf("ghi khoá ai_providers.%s: %w", id, err)
	}
	return nil
}

// SearchModels trả một trang danh mục: lọc + keyset + limit trong SQL.
func (s *AIConnStore) SearchModels(ctx context.Context, providerID string, q aiconn.ModelQuery) (aiconn.ModelPage, error) {
	rows, err := s.q.SearchAiModels(ctx, gen.SearchAiModelsParams{
		ProviderID: providerID,
		Column2:    q.Q,
		Column3:    q.Filter,
		Column4:    q.OnlyID,
		Column5:    q.After,
		Limit:      int32(q.Limit),
	})
	if err != nil {
		return aiconn.ModelPage{}, fmt.Errorf("đọc ai_models của %s: %w", providerID, err)
	}
	models := make([]domain.AIModel, 0, len(rows))
	for _, r := range rows {
		models = append(models, modelToDomain(r))
	}
	return aiconn.ModelPage{Models: models}, nil
}

// CountModels đếm model khớp bộ lọc (cho `total` của trang).
func (s *AIConnStore) CountModels(ctx context.Context, providerID string, q aiconn.ModelQuery) (int, error) {
	n, err := s.q.CountAiModels(ctx, gen.CountAiModelsParams{
		ProviderID: providerID,
		Column2:    q.Q,
		Column3:    q.Filter,
		Column4:    q.OnlyID,
	})
	if err != nil {
		return 0, fmt.Errorf("đếm ai_models của %s: %w", providerID, err)
	}
	return int(n), nil
}

// SetSelected ghi danh sách model đã chọn.
func (s *AIConnStore) SetSelected(ctx context.Context, id string, selected []string) error {
	raw, err := json.Marshal(selected)
	if err != nil {
		return err
	}
	return s.q.SetAiProviderSelected(ctx, gen.SetAiProviderSelectedParams{ID: id, Selected: raw})
}

// SetStatus ghi trạng thái thẻ.
func (s *AIConnStore) SetStatus(ctx context.Context, id string, st domain.AIStatus) error {
	return s.q.SetAiProviderStatus(ctx, gen.SetAiProviderStatusParams{ID: id, Status: st})
}

// DeleteProvider xoá thẻ (ai_models xoá theo cascade).
func (s *AIConnStore) DeleteProvider(ctx context.Context, id string) error {
	return s.q.DeleteAiProvider(ctx, id)
}

// ListModels trả danh mục đã lưu của provider.
func (s *AIConnStore) ListModels(ctx context.Context, providerID string) ([]domain.AIModel, error) {
	rows, err := s.q.ListAiModels(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("đọc ai_models của %s: %w", providerID, err)
	}
	out := make([]domain.AIModel, 0, len(rows))
	for _, r := range rows {
		out = append(out, modelToDomain(r))
	}
	return out, nil
}

// GetModel trả một model; ok=false khi vắng.
func (s *AIConnStore) GetModel(ctx context.Context, providerID, modelID string) (domain.AIModel, bool, error) {
	r, err := s.q.GetAiModel(ctx, gen.GetAiModelParams{ProviderID: providerID, ID: modelID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AIModel{}, false, nil
	}
	if err != nil {
		return domain.AIModel{}, false, fmt.Errorf("đọc ai_models.%s/%s: %w", providerID, modelID, err)
	}
	return modelToDomain(r), true, nil
}

// ReplaceCatalog đổi toàn bộ danh mục tự động trong một transaction: xoá
// model không-manual, upsert từng model mới, đặt catalog_at. Model thêm tay
// sống sót.
func (s *AIConnStore) ReplaceCatalog(ctx context.Context, providerID string, models []domain.AIModel, at time.Time) error {
	if s.pool == nil {
		return errors.New("ReplaceCatalog cần pool (transaction)")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("mở transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	q := gen.New(tx)
	if err := q.DeleteAiCatalogAuto(ctx, providerID); err != nil {
		return fmt.Errorf("dọn danh mục cũ: %w", err)
	}
	for _, m := range models {
		if err := q.UpsertAiModel(ctx, upsertParams(providerID, m)); err != nil {
			return fmt.Errorf("ghi model %s: %w", m.ID, err)
		}
	}
	if err := q.SetAiProviderCatalogAt(ctx, gen.SetAiProviderCatalogAtParams{
		ID:        providerID,
		CatalogAt: pgtype.Timestamptz{Time: at, Valid: true},
	}); err != nil {
		return fmt.Errorf("ghi catalog_at: %w", err)
	}
	return tx.Commit(ctx)
}

// UpsertModel ghi một model (thêm tay hoặc cập nhật).
func (s *AIConnStore) UpsertModel(ctx context.Context, providerID string, m domain.AIModel) error {
	return s.q.UpsertAiModel(ctx, upsertParams(providerID, m))
}

func upsertParams(providerID string, m domain.AIModel) gen.UpsertAiModelParams {
	caps, _ := json.Marshal(m.Caps)
	return gen.UpsertAiModelParams{
		ProviderID:  providerID,
		ID:          m.ID,
		Caps:        caps,
		ContextK:    pgtype.Int4{Int32: int32(m.ContextK), Valid: m.ContextK > 0},
		InPerM:      pgtype.Float8{Float64: m.InPerM, Valid: m.InPerM > 0},
		OutPerM:     pgtype.Float8{Float64: m.OutPerM, Valid: m.OutPerM > 0},
		PerImage:    pgtype.Float8{Float64: m.PerImage, Valid: m.PerImage > 0},
		Recommended: m.Recommended,
		Manual:      m.Manual,
	}
}
