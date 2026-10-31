package editor

import "math"

func commandObjectArray(value any) bool {
	switch entries := value.(type) {
	case []Object:
		for _, entry := range entries {
			if entry == nil {
				return false
			}
		}
		return true
	case []any:
		for _, entry := range entries {
			if object, ok := entry.(map[string]any); !ok || object == nil {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// Engine schemas reject malformed JSON before mutation. These boundary checks
// are separate from the fill applicator's trusted, generated command batches.
func validateCLICommandShape(command Command, index int) error {
	params := command.Params
	fail := func(message, path string) error { return invalidCommand(message, index, path) }
	if params == nil || command.Type == "" {
		return fail("Lệnh phải có {type, params}", "")
	}
	stringField := func(key string) error {
		if _, ok := params[key].(string); !ok {
			return fail("Trường "+key+" phải là chuỗi", key)
		}
		return nil
	}
	objectField := func(key string) error {
		if value, ok := params[key].(map[string]any); !ok || value == nil {
			return fail("Trường "+key+" phải là đối tượng JSON", key)
		}
		return nil
	}
	booleanField := func(object Object, key, path string) error {
		if value, exists := object[key]; exists {
			if _, ok := value.(bool); !ok {
				return fail("Trường "+path+" phải là boolean", path)
			}
		}
		return nil
	}
	switch command.Type {
	case "track.add":
		allowed := map[string]bool{}
		for _, key := range []string{
			"id", "beforeTrackId", "name", "height", "locked", "visible", "muted", "solo", "syncLock",
			"parentTrackId", "isGroup", "isCollapsed", "kind", "order", "templateSlot", "templateLoudnessLufs",
			"templateScenes", "templateBackdrop", "templateBackgroundAppearance",
		} {
			allowed[key] = true
		}
		for key, value := range params {
			if !allowed[key] {
				return fail("Trường track không được hỗ trợ", key)
			}
			switch key {
			case "id", "name", "kind":
				if _, ok := value.(string); !ok {
					return fail("Trường track phải là chuỗi", key)
				}
			case "beforeTrackId", "parentTrackId":
				if value != nil {
					if _, ok := value.(string); !ok {
						return fail("Mã track không hợp lệ", key)
					}
				}
			case "locked", "visible", "muted", "solo", "syncLock", "isGroup", "isCollapsed":
				if err := booleanField(params, key, key); err != nil {
					return err
				}
			case "height", "order":
				number := fillNum(params, key, math.NaN())
				if !fillFinite(number) || (key == "height" && number <= 0) {
					return fail("Kích thước hoặc thứ tự track không hợp lệ", key)
				}
			}
		}
	case "item.add":
		return objectField("item")
	case "track.update", "item.update":
		if err := stringField("id"); err != nil {
			return err
		}
		return objectField("updates")
	case "media.add":
		return objectField("media")
	case "media.update":
		if err := stringField("mediaId"); err != nil {
			return err
		}
		if err := objectField("updates"); err != nil {
			return err
		}
		updates := fillObject(params["updates"])
		if tags, exists := updates["tags"]; exists {
			switch values := tags.(type) {
			case []string:
			case []any:
				for _, value := range values {
					if _, ok := value.(string); !ok {
						return fail("Nhãn media phải là chuỗi", "updates.tags")
					}
				}
			default:
				return fail("Nhãn media phải là danh sách", "updates.tags")
			}
		}
	case "caption.setWords":
		if err := stringField("itemId"); err != nil {
			return err
		}
		if !commandObjectArray(params["words"]) {
			return fail("Danh sách từ không hợp lệ", "words")
		}
		if err := booleanField(params, "estimated", "estimated"); err != nil {
			return err
		}
		for _, word := range fillObjects(params["words"]) {
			if err := booleanField(word, "estimated", "words.estimated"); err != nil {
				return err
			}
		}
	case "caption.setPreset":
		if err := stringField("itemId"); err != nil {
			return err
		}
		return stringField("preset")
	case "caption.updateWord":
		if err := stringField("itemId"); err != nil {
			return err
		}
		return objectField("word")
	case "caption.importTimed":
		if err := objectField("data"); err != nil {
			return err
		}
		data := fillObject(params["data"])
		if !commandObjectArray(data["words"]) || !commandObjectArray(data["cues"]) {
			return fail("Dữ liệu phụ đề không hợp lệ", "data")
		}
		if _, ok := data["estimated"].(bool); !ok {
			return fail("Thiếu trạng thái ước lượng", "data.estimated")
		}
	}
	return nil
}
