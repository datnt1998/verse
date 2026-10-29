package llmcontract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
)

// EncodeArgs dựng tham số tool từ một giá trị miền (domain struct) theo đúng
// hình dạng schema strict yêu cầu: thuộc tính nào schema bắt buộc mà giá trị
// bỏ trống (omitempty/zero) thì điền null, miễn schema cho phép null.
//
// Bản ghi lưu trữ bỏ trống trường tùy chọn, còn hợp đồng tool strict bắt mọi
// thuộc tính phải có mặt, nên hai hình dạng khác nhau đúng ở các trường đó.
// Hàm đọc yêu cầu từ chính schema để không sinh định nghĩa hình dạng thứ hai.
func EncodeArgs(schema map[string]any, value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal giá trị: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("phân tích giá trị: %w", err)
	}
	filled, err := fillRequired(schema, decoded, "$")
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(filled)
	if err != nil {
		return nil, fmt.Errorf("marshal tham số: %w", err)
	}
	return out, nil
}

func fillRequired(schema map[string]any, value any, path string) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case map[string]any:
		properties, _ := schema["properties"].(map[string]any)
		required, err := requiredNames(schema["required"])
		if err != nil {
			return nil, fmt.Errorf("%s.required: %w", path, err)
		}
		for _, name := range required {
			rawProp, ok := properties[name]
			if !ok {
				continue
			}
			prop, ok := rawProp.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s.%s: hợp đồng không hợp lệ", path, name)
			}
			child, exists := typed[name]
			if !exists {
				// Giá trị zero của Go bị omitempty bỏ khỏi JSON, còn hợp đồng
				// strict bắt thuộc tính phải có mặt: null nếu cho phép null,
				// ngược lại là giá trị rỗng đúng kiểu schema khai báo.
				zero, err := zeroValue(prop, path+"."+name)
				if err != nil {
					return nil, err
				}
				typed[name] = zero
				continue
			}
			filled, err := fillRequired(prop, child, path+"."+name)
			if err != nil {
				return nil, err
			}
			typed[name] = filled
		}
		return typed, nil
	case []any:
		items, ok := schema["items"].(map[string]any)
		if !ok {
			return typed, nil
		}
		for i, item := range typed {
			filled, err := fillRequired(items, item, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			typed[i] = filled
		}
		return typed, nil
	default:
		return value, nil
	}
}

// zeroValue là giá trị rỗng đúng kiểu mà schema khai báo cho một thuộc tính
// bắt buộc bị bỏ trống: null khi schema cho phép null, còn lại là [] / {} /
// "" / 0 / false theo kiểu đầu tiên tìm thấy.
func zeroValue(schema map[string]any, path string) (any, error) {
	types, err := schemaTypes(schema["type"])
	if err != nil {
		return nil, fmt.Errorf("%s.type: %w", path, err)
	}
	if slices.Contains(types, "null") {
		return nil, nil
	}
	for _, name := range types {
		switch name {
		case "array":
			return []any{}, nil
		case "object":
			return fillRequired(schema, map[string]any{}, path)
		case "string":
			return "", nil
		case "integer", "number":
			return 0, nil
		case "boolean":
			return false, nil
		}
	}
	return nil, fmt.Errorf("%s: không suy ra được giá trị rỗng từ kiểu %v", path, types)
}
