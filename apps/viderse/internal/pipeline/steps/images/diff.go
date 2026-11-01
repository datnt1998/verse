package images

import "strings"

// DiffSegments là một mảnh của so sánh từ-đến-từ giữa hai bản prompt.
type diffOp int

const (
	diffSame diffOp = iota
	diffInsert
	diffDelete
)

type diffSegment struct {
	op   diffOp
	text string
}

// DiffText so hai nguyên văn theo từ và trả các mảnh đánh dấu giữ/thêm/bỏ.
// Mỗi mảnh giữ nguyên khoảng trắng của bản gốc để ghép lại đúng câu.
func DiffText(before, after string) [][2]string {
	a := splitWords(before)
	b := splitWords(after)
	ops := lcsOps(a, b)
	out := make([][2]string, 0, len(ops))
	for _, segment := range mergeSegments(ops) {
		mark := ""
		switch segment.op {
		case diffInsert:
			mark = "ins"
		case diffDelete:
			mark = "del"
		}
		out = append(out, [2]string{segment.text, mark})
	}
	return out
}

// splitWords cắt văn bản thành các từ kèm khoảng trắng đứng trước, để ghép lại
// đúng nguyên văn ban đầu.
func splitWords(text string) []string {
	if text == "" {
		return nil
	}
	out := make([]string, 0, len(text)/5+1)
	start := 0
	for index := 0; index < len(text); {
		if text[index] == ' ' || text[index] == '\n' || text[index] == '\t' {
			index++
			continue
		}
		// Từ kéo tới khoảng trắng kế tiếp; khoảng trắng đứng trước đi kèm từ.
		next := index
		for next < len(text) && text[next] != ' ' && text[next] != '\n' && text[next] != '\t' {
			next++
		}
		out = append(out, text[start:next])
		start = next
		index = next
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

// lcsOps tính dãy giữ/thêm/bỏ theo dãy con chung dài nhất.
func lcsOps(a, b []string) []diffSegment {
	n, m := len(a), len(b)
	table := make([][]int32, n+1)
	for i := range table {
		table[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				table[i][j] = table[i+1][j+1] + 1
				continue
			}
			if table[i+1][j] >= table[i][j+1] {
				table[i][j] = table[i+1][j]
			} else {
				table[i][j] = table[i][j+1]
			}
		}
	}
	out := make([]diffSegment, 0, n+m)
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			out = append(out, diffSegment{op: diffSame, text: a[i]})
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			out = append(out, diffSegment{op: diffDelete, text: a[i]})
			i++
		default:
			out = append(out, diffSegment{op: diffInsert, text: b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, diffSegment{op: diffDelete, text: a[i]})
	}
	for ; j < m; j++ {
		out = append(out, diffSegment{op: diffInsert, text: b[j]})
	}
	return out
}

// mergeSegments gộp các mảnh liền nhau cùng loại thành một mảnh văn bản.
func mergeSegments(segments []diffSegment) []diffSegment {
	out := make([]diffSegment, 0, len(segments))
	for _, segment := range segments {
		if last := len(out) - 1; last >= 0 && out[last].op == segment.op {
			out[last].text += segment.text
			continue
		}
		out = append(out, segment)
	}
	for index := range out {
		if strings.TrimSpace(out[index].text) == "" {
			// Mảnh chỉ có khoảng trắng thuộc về mảnh trước cho dễ đọc.
			if index > 0 {
				out[index-1].text += out[index].text
				out[index].text = ""
				continue
			}
			if index+1 < len(out) {
				out[index+1].text = out[index].text + out[index+1].text
				out[index].text = ""
			}
		}
	}
	kept := out[:0]
	for _, segment := range out {
		if segment.text == "" {
			continue
		}
		kept = append(kept, segment)
	}
	return kept
}
