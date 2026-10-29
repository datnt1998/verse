package commands

import (
	"errors"
	"strconv"
	"strings"
)

// ErrNotCommand 表示输入不是以 "/" 开头的斜杠命令。
var ErrNotCommand = errors.New("không phải lệnh")

// ErrUnterminatedQuote 表示参数里有没闭合的引号。
var ErrUnterminatedQuote = errors.New("thiếu dấu ngoặc kép đóng")

// UnknownError 表示命令名/别名未注册。
type UnknownError struct {
	Name string
}

func (e *UnknownError) Error() string {
	return "Lệnh không xác định: /" + e.Name
}

// UsageError 是语法/取值错误；消息即 G13 要求的 "Cách dùng: …"。
type UsageError struct {
	Spec Spec
}

func (e *UsageError) Error() string {
	return "Cách dùng: " + e.Spec.Usage
}

// Invocation 是解析后的调用。Values 保存位置/前缀参数的值（文本参数保留空格），
// Flags 以参数 Key 标记开关是否出现。
type Invocation struct {
	Name   string
	Spec   Spec
	Args   []string
	Values map[string]string
	Flags  map[string]bool
}

// SplitArgs 按空白切分参数，支持 "…" 与 '…'（G12）。引号内的反斜杠可转义
// 当前引号或反斜杠；引号外的反斜杠保持原样（兼容 Windows 路径）。
func SplitArgs(s string) ([]string, error) {
	var (
		out     []string
		b       strings.Builder
		inToken bool
		quote   byte
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			switch {
			case c == '\\' && i+1 < len(s) && (s[i+1] == quote || s[i+1] == '\\'):
				b.WriteByte(s[i+1])
				i++
			case c == quote:
				quote = 0
			default:
				b.WriteByte(c)
			}
			continue
		}
		switch c {
		case ' ', '\t', '\r', '\n':
			if inToken {
				out = append(out, b.String())
				b.Reset()
				inToken = false
			}
		case '"', '\'':
			quote = c
			inToken = true
		default:
			b.WriteByte(c)
			inToken = true
		}
	}
	if quote != 0 {
		return nil, ErrUnterminatedQuote
	}
	if inToken {
		out = append(out, b.String())
	}
	return out, nil
}

// Split 把一行拆成命令名与参数 token（不做语法校验）。
// 非斜杠行或只有 "/" 时返回 ErrNotCommand。
func Split(line string) (string, []string, error) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "/") {
		return "", nil, ErrNotCommand
	}
	body := strings.TrimSpace(strings.TrimPrefix(line, "/"))
	if body == "" {
		return "", nil, ErrNotCommand
	}
	name, rest := cutName(body)
	args, err := SplitArgs(rest)
	if err != nil {
		return "", nil, err
	}
	return name, args, nil
}

// Parse 解析并校验一行斜杠命令。语法或取值错误返回 *UsageError，
// 命令不存在返回 *UnknownError，输入不是命令返回 ErrNotCommand。
func Parse(line string) (Invocation, error) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "/") {
		return Invocation{}, ErrNotCommand
	}
	body := strings.TrimSpace(strings.TrimPrefix(line, "/"))
	name, rest := cutName(body)
	if name == "" {
		return Invocation{}, &UnknownError{}
	}
	spec, ok := Find(name)
	if !ok {
		return Invocation{}, &UnknownError{Name: name}
	}
	args, err := SplitArgs(rest)
	if err != nil {
		return Invocation{}, &UsageError{Spec: spec}
	}
	values, flags, ok := bind(spec, args)
	if !ok {
		return Invocation{}, &UsageError{Spec: spec}
	}
	return Invocation{Name: spec.Name, Spec: spec, Args: args, Values: values, Flags: flags}, nil
}

// cutName 取第一个空白之前的命令名，返回小写名与其余参数文本。
func cutName(body string) (string, string) {
	body = strings.TrimLeft(body, " \t\r\n")
	if i := strings.IndexAny(body, " \t\r\n"); i >= 0 {
		return strings.ToLower(body[:i]), strings.TrimLeft(body[i:], " \t\r\n")
	}
	return strings.ToLower(body), ""
}

// bind 按 Spec.Params 绑定参数并校验；失败返回 ok=false（调用方转成 UsageError）。
func bind(spec Spec, args []string) (map[string]string, map[string]bool, bool) {
	values := make(map[string]string, len(spec.Params))
	flags := make(map[string]bool)
	positional := make([]Param, 0, len(spec.Params))
	for _, p := range spec.Params {
		if p.Kind != KindFlag && p.Prefix == "" {
			positional = append(positional, p)
		}
	}

	pos := 0
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "" {
			continue
		}
		if p, ok := flagParam(spec, arg); ok {
			flags[p.Key] = true
			continue
		}
		if p, value, ok := prefixParam(spec, arg); ok {
			if p.Kind == KindText {
				text := strings.TrimSpace(strings.Join(append([]string{value}, args[i+1:]...), " "))
				if text == "" {
					return nil, nil, false
				}
				values[p.Key] = text
				i = len(args)
			} else {
				if strings.TrimSpace(value) == "" {
					return nil, nil, false
				}
				values[p.Key] = value
			}
			continue
		}
		if strings.HasPrefix(arg, "--") {
			return nil, nil, false
		}
		if pos >= len(positional) {
			return nil, nil, false
		}
		p := positional[pos]
		if p.Kind == KindText {
			values[p.Key] = strings.TrimSpace(strings.Join(args[i:], " "))
			i = len(args)
		} else {
			values[p.Key] = arg
		}
		pos++
	}

	for _, p := range spec.Params {
		if p.Kind == KindFlag {
			continue
		}
		value := strings.TrimSpace(values[p.Key])
		if value == "" {
			if p.Required {
				return nil, nil, false
			}
			continue
		}
		switch p.Kind {
		case KindEnum:
			if !hasOption(p.Options, value) {
				return nil, nil, false
			}
		case KindNum:
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return nil, nil, false
			}
		}
	}
	return values, flags, true
}

func flagParam(spec Spec, arg string) (Param, bool) {
	for _, p := range spec.Params {
		if p.Kind == KindFlag && p.Flag != "" && arg == p.Flag {
			return p, true
		}
	}
	return Param{}, false
}

func prefixParam(spec Spec, arg string) (Param, string, bool) {
	for _, p := range spec.Params {
		if p.Prefix == "" || !strings.HasPrefix(arg, p.Prefix) {
			continue
		}
		return p, strings.TrimPrefix(arg, p.Prefix), true
	}
	return Param{}, "", false
}
