package web

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/datnt1998/noverse/internal/utils"
)

// jsonBodyMessage 是请求体 JSON 解码失败时回给用户的越南语 400 文案（合约：错误一律越南语）。
func jsonBodyMessage(err error) string {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return fmt.Sprintf("Dữ liệu gửi lên quá lớn (tối đa %d byte)", tooBig.Limit)
	}
	return utils.DescribeJSONError(err)
}
