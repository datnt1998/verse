package backdrop

import "net/http"

// HTTPDoer là mặt tối thiểu của http.Client mà gói này cần; test tiêm doer
// giả nên không có lời gọi mạng nào.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}
