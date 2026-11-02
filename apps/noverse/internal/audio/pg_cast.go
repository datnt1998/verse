package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// Cast storage on Postgres: one audio_cast row per speaker (role_key = speaker
// ID) plus a reserved role_key "__config__" carrying Mode/Single/NarratorCharacterID.
const pgCastConfigKey = "__config__"

type pgCastConfig struct {
	Mode                string     `json:"mode"`
	VoiceMode           string     `json:"voiceMode,omitempty"`
	NarratorCharacterID string     `json:"narratorCharacterId,omitempty"`
	Single              SingleCast `json:"single"`
}

// pgSpeakerPayload is the non-voice part of a speaker row.
type pgSpeakerPayload struct {
	Name string `json:"name"`
	Ini  string `json:"ini,omitempty"`
	Role string `json:"role"`
	Kind string `json:"kind,omitempty"`
	// VoiceID is the file-backend voice string (audio_voices.legacy_id); the
	// row's voice_id column is the resolved FK and stays authoritative.
	VoiceID *string `json:"voiceId,omitempty"`
	SameAs  string  `json:"sameAs,omitempty"`
}

func (s *PGService) loadCast(ctx context.Context) (*Cast, int64, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return nil, 0, pgErr(err)
	}
	return s.loadCastFor(ctx, scope)
}

// loadCastFor assembles the cast for an explicit scope (the export runner
// works on the job's story, not the cached service scope).
func (s *PGService) loadCastFor(ctx context.Context, scope postgres.ScopeRef) (*Cast, int64, error) {
	rows, err := s.deps.Audio.ListCast(ctx, scope)
	if err != nil {
		return nil, 0, pgErr(err)
	}
	voices, err := s.deps.Audio.ListVoices(ctx, scope.ProjectID)
	if err != nil {
		return nil, 0, pgErr(err)
	}
	return castFromRows(rows, voices)
}

func castFromRows(rows []postgres.CastRole, voices []postgres.Voice) (*Cast, int64, error) {
	legacyByID := map[string]string{}
	for _, v := range voices {
		legacyByID[postgres.UUIDString(v.ID)] = v.LegacyID
	}
	cfg := pgCastConfig{Mode: ModeCast, Single: SingleCast{DialogueShift: true, KeepTones: true}}
	cast := &Cast{Speakers: []Speaker{}}
	var maxVersion int64
	for _, row := range rows {
		if row.Version > maxVersion {
			maxVersion = row.Version
		}
		if row.RoleKey == pgCastConfigKey {
			var c pgCastConfig
			if err := json.Unmarshal(row.Payload, &c); err == nil {
				if c.Mode != "" {
					cfg = c
				}
			}
			continue
		}
		var sp pgSpeakerPayload
		if err := json.Unmarshal(row.Payload, &sp); err != nil {
			return nil, 0, fmt.Errorf("audio cast %q hỏng: %w", row.RoleKey, err)
		}
		speaker := Speaker{ID: row.RoleKey, Name: sp.Name, Ini: sp.Ini, Role: sp.Role, Kind: sp.Kind, SameAs: sp.SameAs}
		if row.VoiceID.Valid {
			legacy := legacyByID[postgres.UUIDString(row.VoiceID)]
			speaker.VoiceID = &legacy
		}
		cast.Speakers = append(cast.Speakers, speaker)
	}
	cast.Mode = cfg.Mode
	cast.VoiceMode = cfg.VoiceMode
	cast.NarratorCharacterID = cfg.NarratorCharacterID
	cast.Single = cfg.Single
	sort.SliceStable(cast.Speakers, func(i, j int) bool {
		if cast.Speakers[i].ID == NarratorID {
			return true
		}
		if cast.Speakers[j].ID == NarratorID {
			return false
		}
		return cast.Speakers[i].ID < cast.Speakers[j].ID
	})
	return cast, maxVersion, nil
}

// voiceRef resolves a file-mode voice id to the audio_voices row.
func (s *PGService) voiceRef(ctx context.Context, projectID pgtype.UUID, legacy string) (pgtype.UUID, error) {
	v, err := s.deps.Audio.VoiceByLegacy(ctx, projectID, legacy)
	if err != nil {
		if errors.Is(err, postgres.ErrJobNotFound) {
			return pgtype.UUID{}, nil
		}
		return pgtype.UUID{}, pgErr(err)
	}
	return v.ID, nil
}

// Cast returns the cast; missing state initializes from the story characters
// and persists it, so the first panel open already has content.
func (s *PGService) Cast(ref CharacterRef) (*Cast, error) {
	ctx := context.Background()
	cast, _, err := s.loadCast(ctx)
	if err != nil {
		return nil, err
	}
	if len(cast.Speakers) > 0 {
		return cast, nil
	}
	fresh := DefaultCast(ref.Characters, ref.Support)
	saved, err := s.saveCast(ctx, &fresh)
	if err != nil {
		return nil, err
	}
	return saved, nil
}

func (s *PGService) saveCast(ctx context.Context, cast *Cast) (*Cast, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return nil, pgErr(err)
	}
	cfg := pgCastConfig{Mode: cast.Mode, VoiceMode: cast.VoiceMode, NarratorCharacterID: cast.NarratorCharacterID, Single: cast.Single}
	if _, err := s.deps.Audio.SaveCast(ctx, scope, pgCastConfigKey, pgtype.UUID{}, cfg); err != nil {
		return nil, pgErr(err)
	}
	keep := map[string]bool{pgCastConfigKey: true}
	for _, sp := range cast.Speakers {
		var voiceID pgtype.UUID
		if sp.VoiceID != nil && *sp.VoiceID != "" {
			voiceID, err = s.voiceRef(ctx, scope.ProjectID, *sp.VoiceID)
			if err != nil {
				return nil, err
			}
		}
		payload := pgSpeakerPayload{Name: sp.Name, Ini: sp.Ini, Role: sp.Role, Kind: sp.Kind, VoiceID: sp.VoiceID, SameAs: sp.SameAs}
		if _, err := s.deps.Audio.SaveCast(ctx, scope, sp.ID, voiceID, payload); err != nil {
			return nil, pgErr(err)
		}
		keep[sp.ID] = true
	}
	rows, err := s.deps.Audio.ListCast(ctx, scope)
	if err != nil {
		return nil, pgErr(err)
	}
	for _, row := range rows {
		if keep[row.RoleKey] {
			continue
		}
		if err := s.deps.Audio.DeleteCast(ctx, scope, row.RoleKey); err != nil {
			return nil, pgErr(err)
		}
	}
	saved, _, err := s.loadCast(ctx)
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// SaveCast persists the cast.
func (s *PGService) SaveCast(c *Cast) (*Cast, error) {
	return s.saveCast(context.Background(), c)
}

// CastVersion is the ETag token: the newest persisted row version.
func (s *PGService) CastVersion() string {
	_, v, err := s.loadCast(context.Background())
	if err != nil {
		return "pg-cast-0"
	}
	return "pg-cast-" + strconv.FormatInt(v, 10)
}

// castForRender reads the cast without initializing it.
func (s *PGService) castForRender(ctx context.Context) (*Cast, error) {
	cast, _, err := s.loadCast(ctx)
	if err != nil {
		return nil, err
	}
	if len(cast.Speakers) == 0 && cast.Mode == "" {
		cast.Mode = ModeCast
	}
	if cast.Mode == "" {
		cast.Mode = ModeCast
	}
	return cast, nil
}

// CastOldSegments counts stale segments per speaker (the "cần đọc lại" badge).
func (s *PGService) CastOldSegments(c *Cast) error {
	ctx := context.Background()
	scope, err := s.resolve(ctx)
	if err != nil {
		return pgErr(err)
	}
	settings := s.Settings()
	voices, err := s.Voices(ctx)
	if err != nil {
		return err
	}
	voiceMap := VoiceMap(voices)
	counts := map[string]int{}
	chapters, err := s.chapterList(ctx, scope)
	if err != nil {
		return err
	}
	for _, ch := range chapters {
		script, _, err := s.loadScript(ctx, scope, ch)
		if err != nil {
			return err
		}
		if script == nil {
			continue
		}
		for _, seg := range script.Segments {
			if seg.Rendered == nil {
				continue
			}
			engine, model := s.engineIdent()
			want := pgSegmentKey(seg, c, voiceMap, settings, engine, model)
			if seg.Rendered.Key != want.Hash() {
				counts[seg.Speaker]++
			}
		}
	}
	for i := range c.Speakers {
		c.Speakers[i].OldSegments = counts[c.Speakers[i].ID]
	}
	return nil
}

// ApplyPackVoice is a no-op on the hosted backend: pack assets are a
// story-directory concern and no pack voice source exists in the tables.
func (s *PGService) ApplyPackVoice(c *Cast) error { return nil }

// ChangeRoleVoice switches one speaker's voice and enqueues update jobs for
// chapters that still reference the old audio. It returns the number of
// stale segments queued for re-synthesis.
func (s *PGService) ChangeRoleVoice(ctx context.Context, role, voiceID, sameAs string, rerenderOld bool) (*Cast, int, error) {
	cast, _, err := s.loadCast(ctx)
	if err != nil {
		return nil, 0, err
	}
	idx := -1
	for i := range cast.Speakers {
		if cast.Speakers[i].ID == role || strings.EqualFold(cast.Speakers[i].Name, role) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, 0, conflict("Không có vai %q trong phân vai.", role)
	}
	sameAs = strings.TrimSpace(sameAs)
	if sameAs != "" {
		found := false
		for _, sp := range cast.Speakers {
			if sp.ID == sameAs {
				found = true
				break
			}
		}
		if !found {
			return nil, 0, conflict("Không tìm thấy vai %q.", sameAs)
		}
	}
	cast.Speakers[idx].SameAs = sameAs
	if voiceID == "" {
		cast.Speakers[idx].VoiceID = nil
	} else {
		id := voiceID
		cast.Speakers[idx].VoiceID = &id
	}
	saved, err := s.saveCast(ctx, cast)
	if err != nil {
		return nil, 0, err
	}
	if !rerenderOld {
		return saved, 0, nil
	}
	queued, err := s.enqueueRoleUpdates(ctx, role)
	if err != nil {
		return nil, 0, err
	}
	return saved, queued, nil
}

// enqueueRoleUpdates queues "update" jobs for chapters where the role still
// has selected renders keyed to the old parameters, and returns the count of
// stale segments across those chapters.
func (s *PGService) enqueueRoleUpdates(ctx context.Context, role string) (int, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return 0, pgErr(err)
	}
	chapters, err := s.chapterList(ctx, scope)
	if err != nil {
		return 0, err
	}
	queued := 0
	for _, ch := range chapters {
		script, _, err := s.loadScript(ctx, scope, ch)
		if err != nil {
			return 0, err
		}
		if script == nil || script.Status != StatusLocked {
			continue
		}
		stale := false
		for _, seg := range script.Segments {
			if seg.Speaker != role {
				continue
			}
			if seg.Rendered != nil {
				stale = true
				queued++
			}
		}
		if !stale {
			continue
		}
		text, err := s.deps.Nar.ChapterText(ctx, scope, ch)
		if err != nil {
			return 0, pgErr(err)
		}
		if _, err := s.Enqueue(ctx, script.Volume, ch, SourceHash(text)); err != nil && !isNoopEnqueue(err) {
			return 0, err
		}
	}
	return queued, nil
}

// Voices merges uploaded project voices with the remote worker's real engine
// catalogue. Hosted has no local engine, so the worker state is the source of
// the presets a cast may reference.
func (s *PGService) Voices(ctx context.Context) ([]ProjectVoice, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return nil, pgErr(err)
	}
	rows, err := s.deps.Audio.ListVoices(ctx, scope.ProjectID)
	if err != nil {
		return nil, pgErr(err)
	}
	out := make([]ProjectVoice, 0, len(rows))
	for _, row := range rows {
		var meta struct {
			Name        string  `json:"name"`
			DurationSec float64 `json:"durationSec"`
			Gender      string  `json:"gender"`
			Note        string  `json:"note"`
		}
		if err := json.Unmarshal(row.Metadata, &meta); err != nil {
			meta.Name = row.LegacyID
		}
		name := meta.Name
		if name == "" {
			name = row.LegacyID
		}
		out = append(out, ProjectVoice{ID: row.LegacyID, Name: name, DurationSec: meta.DurationSec, Gender: meta.Gender, Note: meta.Note})
	}
	if s.deps.RemoteVoices != nil {
		out = MergeVoices(out, s.deps.RemoteVoices(ctx))
	}
	return out, nil
}

// VoiceSample resolves a project voice sample blob (audio_voices.sample_blob_id).
func (s *PGService) VoiceSample(ctx context.Context, id string) (Object, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return Object{}, pgErr(err)
	}
	v, err := s.deps.Audio.VoiceByLegacy(ctx, scope.ProjectID, id)
	if err != nil {
		return Object{}, ErrNotFound
	}
	if !v.SampleBlobID.Valid {
		return Object{}, ErrNotFound
	}
	blob, err := s.deps.Audio.BlobByID(ctx, scope.ProjectID, v.SampleBlobID)
	if err != nil {
		return Object{}, pgErr(err)
	}
	return s.objectFor(blob, "mau-giong.wav"), nil
}
