package eval

import (
	"fmt"
	"strings"
)

// gradeContextExpect 断言 novel_context 交到模型手里的结果。每个串都必须出现在
// 某一次调用的结果里：模型没看到的东西，正文里写不出来——把"没读到"当通过正是
// 这类断言最容易产生的假通过。
func gradeContextExpect(c Case, col Collected, res *Result) {
	needles := c.Expect.ContextContains
	if len(needles) == 0 {
		return
	}
	if len(col.ContextResults) == 0 {
		detail := "phiên chạy không có kết quả novel_context nào để đối chiếu"
		if col.ContextMissing > 0 {
			detail = fmt.Sprintf("phiên chạy không có kết quả novel_context nào để đối chiếu (%d kết quả bị compact/lỗi, không đo được)", col.ContextMissing)
		}
		res.addHardFail("context", detail)
		return
	}
	haystack := strings.Join(col.ContextResults, "\n")
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			res.addPass("context:contains", fmt.Sprintf("novel_context có %q", needle))
			continue
		}
		res.addHardFail("context:contains", fmt.Sprintf("novel_context không có %q", needle))
	}
}
