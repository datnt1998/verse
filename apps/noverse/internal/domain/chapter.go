package domain

import (
	"fmt"
	"unicode"
)

// ReviewInterval 全局审阅间隔（每 N 章触发一次）。
const ReviewInterval = 5

// ShouldReview 根据已完成章节数判断是否需要全局审阅（短篇/中篇模式）。
func ShouldReview(completedCount int) (bool, string) {
	if completedCount > 0 && completedCount%ReviewInterval == 0 {
		return true, fmt.Sprintf("Đã hoàn thành %d chương, kích hoạt thẩm duyệt toàn cục", completedCount)
	}
	return false, ""
}

// ShouldArcReview 长篇模式下判断是否需要弧级/卷级评审。
func ShouldArcReview(isArcEnd, isVolumeEnd bool, volume, arc int) (bool, string) {
	if isVolumeEnd {
		return true, fmt.Sprintf("Tập %d cung %d kết thúc (hết tập), kích hoạt thẩm duyệt cấp cung + cấp tập", volume, arc)
	}
	if isArcEnd {
		return true, fmt.Sprintf("Tập %d cung %d kết thúc, kích hoạt thẩm duyệt cấp cung", volume, arc)
	}
	return false, ""
}

// WordCount 按词计数：以空白分隔、且至少含一个字母或数字的片段各算一个词。
//
// 旧实现按 rune 计字（一个汉字 = 1）。越南语正文明写「chữ」即词：一个音节就是一个
// 空白分隔的 token，故 1 词 ≈ 1 个汉字的篇幅（详见 local://wordcount-handoff.md）。
// 纯标点/分隔符片段（"—"、"…"、"***"）不计入。
func WordCount(content string) int {
	n := 0
	inToken, hasLetterOrDigit := false, false
	for _, r := range content {
		if unicode.IsSpace(r) {
			if inToken && hasLetterOrDigit {
				n++
			}
			inToken, hasLetterOrDigit = false, false
			continue
		}
		inToken = true
		if !hasLetterOrDigit && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			hasLetterOrDigit = true
		}
	}
	if inToken && hasLetterOrDigit {
		n++
	}
	return n
}
