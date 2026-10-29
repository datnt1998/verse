package web

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/datnt1998/noverse/internal/audio"
	"github.com/datnt1998/noverse/internal/library"
)

func projectVoiceSamples(voices []audio.ProjectVoice, base string) {
	for i := range voices {
		voices[i].SampleURL = base + "/" + url.PathEscape(voices[i].ID) + "/sample"
	}
}

func serveVoiceSample(w http.ResponseWriter, r *http.Request, layout audio.Layout, id string) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	wav, err := audio.VoiceSample(ctx, layout, audioSynthesizer(), id)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Không phát được mẫu giọng này. Hãy thử lại hoặc chọn giọng khác.")
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, "mau-giong.wav", time.Time{}, bytes.NewReader(wav))
}

func (g *Gateway) serveStoryVoices(w http.ResponseWriter, r *http.Request, st library.Story, svc audioBackend, segs []string) bool {
	if len(segs) == 2 && segs[1] == "sample" && r.Method == http.MethodGet {
		obj, err := svc.VoiceSample(r.Context(), segs[0])
		if err != nil {
			audioError(w, err)
			return true
		}
		serveAudioObject(w, r, obj)
		return true
	}
	if len(segs) != 0 {
		return false
	}
	switch r.Method {
	case http.MethodGet:
		voices, err := svc.Voices(r.Context())
		if err != nil {
			audioError(w, err)
			return true
		}
		projectVoiceSamples(voices, "/api/projects/"+url.PathEscape(st.Project)+"/voices")
		writeJSON(w, http.StatusOK, voices)
	case http.MethodPost:
		if fb, ok := svc.(*fileAudioBackend); ok {
			g.uploadProjectVoice(w, r, fb.svc.Layout())
			return true
		}
		audioError(w, errAudioUnsupportedPG("tải mẫu giọng"))
	default:
		return false
	}
	return true
}

func (g *Gateway) serveAudioRoleVoice(w http.ResponseWriter, r *http.Request, svc audioBackend, role string) {
	if !checkIfMatchV(w, r, svc.CastVersion(), func() (any, error) {
		c, err := svc.Cast(audio.CharacterRef{})
		return versionedCast{c, svc.CastVersion()}, err
	}) {
		return
	}
	var in struct {
		VoiceID     string `json:"voiceId"`
		SameAs      string `json:"sameAs"`
		RerenderOld bool   `json:"rerenderOld"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	cast, n, err := svc.ChangeRoleVoice(r.Context(), role, in.VoiceID, in.SameAs, in.RerenderOld)
	if err != nil {
		audioError(w, err)
		return
	}
	if err := svc.CastOldSegments(cast); err != nil {
		audioError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		versionedCast
		QueuedSegments int `json:"queuedSegments"`
	}{versionedCast{cast, writeVersionToken(w, svc.CastVersion())}, n})
}
