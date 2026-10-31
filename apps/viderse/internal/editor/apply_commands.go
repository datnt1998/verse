package editor

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/datnt1998/verse/platform/blobstore"
)

type ApplyCommandsOptions struct {
	DryRun       bool   `json:"dryRun,omitempty"`
	BaseRevision string `json:"baseRevision,omitempty"`
}

func invalidCommand(message string, index any, path string) error {
	issue := Object{"commandIndex": index, "message": message}
	if path != "" {
		issue["path"] = path
	}
	return apiError("invalid_command", message, Object{"commandIndex": index, "issues": []Object{issue}})
}

// ParseCommands validates the untyped JSON boundary before engine execution.
func ParseCommands(value any) ([]Command, error) {
	raw, ok := value.([]any)
	if !ok {
		return nil, invalidCommand("Chuỗi lệnh phải là một mảng JSON", nil, "")
	}
	commands := make([]Command, 0, len(raw))
	for i, value := range raw {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, invalidCommand("Lệnh phải có {type, params}", i, "")
		}
		typ, ok := object["type"].(string)
		if !ok {
			return nil, invalidCommand("Lệnh phải có {type, params}", i, "")
		}
		params, ok := object["params"].(map[string]any)
		if !ok || params == nil {
			return nil, invalidCommand("Tham số lệnh phải là đối tượng JSON", i, "params")
		}
		commands = append(commands, Command{Type: typ, Params: params})
	}
	return commands, nil
}

// RunCommands commits a detached batch with project CAS, then updates independent
// workspace metadata. metadata_partial describes an already committed project;
// callers must not retry it as an uncommitted transaction.
func (s Service) RunCommands(ctx context.Context, id string, commands []Command, options ApplyCommandsOptions) (Object, error) {
	if err := ValidateID(id); err != nil {
		return nil, err
	}
	result, changes, err := s.runCommandBatch(ctx, id, commands, options)
	if err != nil {
		return nil, err
	}
	if options.DryRun {
		return result, nil
	}
	applied, failed := []string{}, []string{}
	failures := []Object{}
	for _, change := range changes {
		if _, err := s.UpdateMediaMetadata(ctx, change.id, change.patch); err != nil {
			failed = append(failed, change.id)
			failures = append(failures, Object{"mediaId": change.id, "error": err.Error()})
		} else {
			applied = append(applied, change.id)
		}
	}
	if len(failed) > 0 {
		return nil, apiError("metadata_partial", "Project đã lưu nhưng một số metadata media chưa được cập nhật", Object{
			"revision": result["revision"], "appliedMediaIds": applied,
			"failedMediaIds": failed, "failures": failures,
		})
	}
	return result, nil
}

func (s Service) runCommandBatch(ctx context.Context, id string, commands []Command, options ApplyCommandsOptions) (Object, []commandMetadataChange, error) {
	// Serialize command batches without holding Store's project lock. Store.Write
	// still performs the authoritative CAS against browser/pipeline saves.
	lock := resourceLock("command-batches/" + id)
	lock.Lock()
	defer lock.Unlock()
	project, base, err := s.Store.Read(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if fillNum(Object(project), "schemaVersion", 0) != SchemaVersion {
		return nil, nil, apiError("unsupported_format", "Phiên bản project không được hỗ trợ")
	}
	if options.BaseRevision != "" {
		if err := CheckRevision(base, options.BaseRevision); err != nil {
			return nil, nil, err
		}
	}
	for index, command := range commands {
		if err := validateCLICommandShape(command, index); err != nil {
			return nil, nil, err
		}
	}
	media, err := s.commandProjectMedia(ctx, project)
	if err != nil {
		return nil, nil, err
	}
	results := make([]any, 0, len(commands))
	var nextMedia []Object
	next, err := applyCommandBatch(project, commands, media, &results, &nextMedia)
	if err != nil {
		return nil, nil, err
	}
	ids, changes, err := s.validateCommandMedia(ctx, next, nextMedia, commands)
	if err != nil {
		return nil, nil, err
	}
	initialState, nextState := commandProjectState(project, media), commandProjectState(next, nextMedia)
	fields := []string{}
	for _, key := range commandStateKeys {
		if !commandJSONEqual(initialState[key], nextState[key]) {
			fields = append(fields, key)
		}
	}
	revision := base
	if !options.DryRun && len(commands) > 0 {
		revision, err = s.Store.Write(ctx, id, next, base)
		if err != nil {
			return nil, nil, err
		}
		if err := s.ReconcileProjectMediaLinks(ctx, id, ids); err != nil {
			return nil, nil, err
		}
	}
	types := make([]string, len(commands))
	for i, command := range commands {
		types[i] = command.Type
	}
	return Object{
		"revision": revision, "baseRevision": base, "dryRun": options.DryRun,
		"summary": Object{"commandCount": len(commands), "commandTypes": types, "changedFields": fields},
		"results": results,
	}, changes, nil
}

type commandMetadataChange struct {
	id    string
	patch Object
}

func (s Service) commandProjectMedia(ctx context.Context, project Project) ([]Object, error) {
	linked, err := s.ListMedia(ctx, fillString(Object(project), "id"))
	if err != nil {
		return nil, err
	}
	records := []Object{}
	seen := map[string]bool{}
	for _, record := range linked {
		value, err := commandMediaMetadata(record)
		if err != nil {
			return nil, err
		}
		records = append(records, value)
		seen[record.ID] = true
	}
	ids := collectCommandMedia(project.Timeline())
	if refs, exists := project["mediaItems"]; exists {
		raw, err := json.Marshal(refs)
		if err != nil {
			return nil, apiError("unsupported_format", "Danh sách media trong project không hợp lệ")
		}
		var entries []Object
		if err := json.Unmarshal(raw, &entries); err != nil || string(raw) == "null" {
			return nil, apiError("unsupported_format", "Danh sách media trong project không hợp lệ")
		}
		for _, ref := range entries {
			id, ok := ref["id"].(string)
			if !ok {
				return nil, apiError("unsupported_format", "Tham chiếu media trong project không hợp lệ")
			}
			ids = commandAppendMediaID(ids, id)
		}
	}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		record, err := s.GetMediaMetadata(ctx, id)
		if err != nil {
			return nil, err
		}
		value, err := commandMediaMetadata(record)
		if err != nil {
			return nil, err
		}
		records = append(records, value)
		seen[id] = true
	}
	return records, nil
}

func (s Service) validateCommandMedia(ctx context.Context, project Project, media []Object, commands []Command) ([]string, []commandMetadataChange, error) {
	ids := collectCommandMedia(project.Timeline())
	for _, record := range media {
		ids = commandAppendMediaID(ids, fillString(record, "id"))
	}
	stored := map[string]Object{}
	for _, id := range ids {
		record, err := s.GetMediaMetadata(ctx, id)
		if err != nil {
			var storageError *Error
			if errors.Is(err, blobstore.ErrNotFound) || (errors.As(err, &storageError) && (storageError.Code == "not_found" || storageError.Code == "invalid_request")) {
				return nil, nil, invalidCommand("Media "+id+" không tồn tại", commandMediaIndex(commands, id), "mediaId")
			}
			return nil, nil, err
		}
		value, err := commandMediaMetadata(record)
		if err != nil {
			return nil, nil, err
		}
		stored[id] = value
	}
	changes := []commandMetadataChange{}
	for _, record := range media {
		id := fillString(record, "id")
		source := stored[id]
		for _, key := range []string{"storageType", "contentHash", "fileName", "fileSize", "mimeType", "duration", "width", "height", "fps", "codec", "audioCodec"} {
			if !commandJSONEqual(record[key], source[key]) {
				return nil, nil, invalidCommand("Không thể sửa dữ liệu nguồn media qua lệnh: "+key, commandMediaIndex(commands, id), key)
			}
		}
		patch := Object{}
		if !commandJSONEqual(record["tags"], source["tags"]) {
			patch["tags"] = record["tags"]
		}
		if !commandJSONEqual(record["attribution"], source["attribution"]) {
			patch["attribution"] = record["attribution"]
		}
		if len(patch) > 0 {
			validated, err := ValidateMediaMetadataPatch(patch)
			if err != nil {
				return nil, nil, invalidCommand(err.Error(), commandMediaIndex(commands, id), "")
			}
			changes = append(changes, commandMetadataChange{id: id, patch: validated})
		}
	}
	return ids, changes, nil
}

func commandMediaMetadata(record MediaRecord) (Object, error) {
	probe := record.Probe
	codec := "unknown"
	if probe.Kind == "video" && probe.VideoCodec != nil {
		codec = *probe.VideoCodec
	} else if probe.Kind == "audio" && probe.AudioCodec != nil {
		codec = *probe.AudioCodec
	}
	fps := 0.0
	if probe.Kind == "video" {
		fps = commandProbeNumber(probe.FPS, 30)
	}
	tags := record.Tags
	if tags == nil {
		tags = []string{}
	}
	out := Object{
		"id": record.ID, "storageType": "workspace", "contentHash": record.ContentHash,
		"fileName": record.FileName, "fileSize": record.Size, "mimeType": record.MIME,
		"duration": commandProbeNumber(probe.DurationSec, 0), "width": commandProbeNumber(probe.Width, 0),
		"height": commandProbeNumber(probe.Height, 0), "fps": fps, "codec": codec, "bitrate": 0,
		"tags": tags, "createdAt": record.CreatedAt, "updatedAt": record.CreatedAt,
	}
	if probe.Kind == "video" && probe.AudioCodec != nil {
		out["audioCodec"] = *probe.AudioCodec
	}
	if attribution := record.Attribution; attribution != nil {
		value := Object{"provider": attribution.Provider}
		if attribution.Author != nil {
			value["author"] = *attribution.Author
		}
		if attribution.AuthorURL != nil {
			value["authorUrl"] = *attribution.AuthorURL
		}
		if attribution.SourceURL != nil {
			value["sourceUrl"] = *attribution.SourceURL
		}
		if attribution.SourceID != nil {
			value["sourceId"] = *attribution.SourceID
		}
		if attribution.License != nil {
			value["license"] = *attribution.License
		}
		out["attribution"] = value
	}
	if background := record.Background; background != nil {
		value := Object{
			"kind": background.Kind, "version": background.Version,
			"detectedAt": background.DetectedAt, "contentHash": background.ContentHash,
		}
		if background.KeyColor != "" {
			value["keyColor"] = background.KeyColor
		}
		out["background"] = value
	}
	if record.Overlay {
		out["overlay"] = true
	}
	return out, nil
}

func commandProbeNumber(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func commandMediaIndex(commands []Command, id string) any {
	encoded, _ := json.Marshal(id) // A string is always JSON serializable.
	for i := len(commands) - 1; i >= 0; i-- {
		raw, err := json.Marshal(commands[i].Params)
		if err == nil && strings.Contains(string(raw), string(encoded)) {
			return i
		}
	}
	return nil
}

func commandAppendMediaID(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func collectCommandMedia(value any) []string {
	ids := []string{}
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			keys := make([]string, 0, len(value))
			for key := range value {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				child := value[key]
				if key == "mediaId" {
					if id, ok := child.(string); ok {
						ids = commandAppendMediaID(ids, id)
						continue
					}
				}
				walk(child)
			}
		case []Object:
			for _, child := range value {
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(value)
	return ids
}

func commandJSONEqual(a, b any) bool {
	x, firstError := json.Marshal(a)
	y, secondError := json.Marshal(b)
	return firstError == nil && secondError == nil && string(x) == string(y)
}

var commandStateKeys = []string{
	"items", "tracks", "transitions", "keyframes", "markers", "compositions", "topLevelSequenceIds",
	"inPoint", "outPoint", "fps", "scrollPosition", "snapEnabled", "currentFrame", "busAudioEq", "masterBusDb",
	"projectName", "projectId", "projectMetadata", "mediaItems", "stashStack", "mainHolder",
}

func commandProjectState(project Project, media []Object) Object {
	timeline := project.Timeline()
	state := Object{}
	for _, key := range []string{"items", "tracks", "transitions", "keyframes", "markers", "compositions", "topLevelSequenceIds"} {
		value := timeline[key]
		if value == nil {
			value = []any{}
		}
		state[key] = value
	}
	state["inPoint"], state["outPoint"] = timeline["inPoint"], timeline["outPoint"]
	state["fps"] = fillNum(project.Metadata(), "fps", 30)
	state["scrollPosition"] = fillNum(timeline, "scrollPosition", 0)
	state["snapEnabled"] = true
	state["currentFrame"] = fillNum(timeline, "currentFrame", 0)
	state["busAudioEq"] = timeline["busAudioEq"]
	state["masterBusDb"] = fillNum(timeline, "masterBusDb", 0)
	state["projectName"], state["projectId"] = project["name"], project["id"]
	state["projectMetadata"], state["mediaItems"] = project["metadata"], media
	state["stashStack"], state["mainHolder"] = []any{}, nil
	return state
}
