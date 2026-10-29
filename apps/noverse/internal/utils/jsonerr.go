package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// DescribeJSONError 把 encoding/json 的解码错误翻成给用户看的越南语短句
// （语法错、类型不符、未知字段、截断）。encoding/json 自身其它 "json: …" 错误统一为
// "JSON không hợp lệ"，不回显英文原文；自定义 UnmarshalJSON 返回的（已是越南语的）错误原样返回。
func DescribeJSONError(err error) string {
	var syntax *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case err == nil:
		return ""
	case errors.Is(err, io.EOF):
		return "Thiếu dữ liệu JSON"
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "JSON không hợp lệ: dữ liệu bị cắt cụt"
	case errors.As(err, &syntax):
		if strings.Contains(syntax.Error(), "unexpected end") {
			return "JSON không hợp lệ: dữ liệu bị cắt cụt"
		}
		return fmt.Sprintf("JSON không hợp lệ: sai cú pháp ở vị trí %d", syntax.Offset)
	case errors.As(err, &typeErr):
		got := jsonValueKinds[strings.Fields(typeErr.Value + " ?")[0]]
		if got == "" {
			got = "giá trị khác"
		}
		if typeErr.Field != "" {
			return fmt.Sprintf("Trường %q sai kiểu: cần %s, nhận %s", typeErr.Field, goKindLabel(typeErr.Type), got)
		}
		return fmt.Sprintf("Dữ liệu sai kiểu: cần %s, nhận %s", goKindLabel(typeErr.Type), got)
	}
	msg := err.Error()
	if field, ok := strings.CutPrefix(msg, "json: unknown field "); ok {
		return "Trường không được hỗ trợ: " + field
	}
	if strings.HasPrefix(msg, "json: ") {
		return "JSON không hợp lệ"
	}
	return msg
}

// jsonValueKinds 对应 UnmarshalTypeError.Value 的 JSON 值类别。
var jsonValueKinds = map[string]string{
	"string": "chuỗi", "number": "số", "bool": "true/false", "object": "đối tượng", "array": "mảng",
}

func goKindLabel(t reflect.Type) string {
	if t == nil {
		return "kiểu khác"
	}
	switch t.Kind() {
	case reflect.String:
		return "chuỗi"
	case reflect.Bool:
		return "true/false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "số nguyên"
	case reflect.Float32, reflect.Float64:
		return "số"
	case reflect.Slice, reflect.Array:
		return "mảng"
	case reflect.Map, reflect.Struct:
		return "đối tượng"
	case reflect.Pointer:
		return goKindLabel(t.Elem())
	}
	return "kiểu khác"
}
