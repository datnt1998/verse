package audio

import "strings"

// 越南语数字读法。Audio 的"bản đọc"必须把数字、日期、金额读成字，模型不负责
// 这件事（同一段文字每次都要得到同一结果），所以这里用纯 Go 实现，输入相同
// 输出必然相同。

var viDigits = [...]string{"không", "một", "hai", "ba", "bốn", "năm", "sáu", "bảy", "tám", "chín"}

// viScales 是三位分组的量级词，索引即分组序号（从低位往高位）。
var viScales = [...]string{"", "nghìn", "triệu", "tỷ", "nghìn tỷ", "triệu tỷ"}

// viNumberWords 把非负整数读成越南语。
//
// 规则（与越南语口语一致）：
//   - 中间分组缺百位要补 "không trăm"（1 045 → "một nghìn không trăm bốn mươi lăm"）
//   - 个位 1 跟在 "mươi" 后读 "mốt"，4 读 "tư"，5 读 "lăm"
//   - 十位为 0 且百位存在时用 "linh"（101 → "một trăm linh một"）
//   - 超出 triệu tỷ 的部分按 "… tỷ" 递归（够用即可，不追求天文数字）
func viNumberWords(n uint64) string {
	if n == 0 {
		return viDigits[0]
	}
	var groups []uint64
	for n > 0 {
		groups = append(groups, n%1000)
		n /= 1000
	}
	var parts []string
	for i := len(groups) - 1; i >= 0; i-- {
		g := groups[i]
		if g == 0 {
			continue
		}
		// 最高位分组之外的分组要补百位，保证 "một nghìn không trăm…" 这类读法。
		forceHundred := i != len(groups)-1
		parts = append(parts, viTripleWords(int(g), forceHundred))
		if scale := viScaleWord(i); scale != "" {
			parts = append(parts, scale)
		}
	}
	return strings.Join(parts, " ")
}

// viScaleWord 返回第 i 个三位分组的量级词；超出预置量级时按 tỷ 递归。
func viScaleWord(i int) string {
	if i < len(viScales) {
		return viScales[i]
	}
	// 第 6 组 = 10^18 = "tỷ tỷ" 起按 3 位一组继续叠加 tỷ，口语极少用到。
	base := viScaleWord(i - 3)
	if base == "" {
		return "tỷ"
	}
	return base + " tỷ"
}

// viTripleWords 读 0..999 的一组。
func viTripleWords(n int, forceHundred bool) string {
	h, t, u := n/100, (n/10)%10, n%10
	var parts []string
	switch {
	case h > 0:
		parts = append(parts, viDigits[h], "trăm")
	case forceHundred:
		parts = append(parts, viDigits[0], "trăm")
	}
	switch {
	case t > 1:
		parts = append(parts, viDigits[t], "mươi")
		switch u {
		case 1:
			parts = append(parts, "mốt")
		case 4:
			parts = append(parts, "tư")
		case 5:
			parts = append(parts, "lăm")
		case 0:
		default:
			parts = append(parts, viDigits[u])
		}
	case t == 1:
		parts = append(parts, "mười")
		switch u {
		case 5:
			parts = append(parts, "lăm")
		case 0:
		default:
			parts = append(parts, viDigits[u])
		}
	case u > 0:
		if h > 0 || forceHundred {
			parts = append(parts, "linh")
		}
		parts = append(parts, viDigits[u])
	}
	return strings.Join(parts, " ")
}

// viDigitsWords 逐位读一个数字串（电话号码、编号、以 0 开头的代码）。
func viDigitsWords(s string) string {
	parts := make([]string, 0, len(s))
	for _, r := range s {
		if r < '0' || r > '9' {
			continue
		}
		parts = append(parts, viDigits[r-'0'])
	}
	return strings.Join(parts, " ")
}

// viOrdinalWord 读 "thứ N" 的 N：1 → nhất，4 → tư，其余按普通数字。
func viOrdinalWord(n uint64) string {
	switch n {
	case 1:
		return "nhất"
	case 4:
		return "tư"
	}
	return viNumberWords(n)
}
