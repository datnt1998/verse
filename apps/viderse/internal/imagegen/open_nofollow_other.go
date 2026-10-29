//go:build !unix

package imagegen

import (
	"errors"
	"os"
)

// openNoFollow trên nền tảng không-unix: Lstat từ chối symlink rồi mở thường.
func openNoFollow(file string) (*os.File, error) {
	st, err := os.Lstat(file)
	if err != nil {
		return nil, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("image_path_scope")
	}
	return os.Open(file)
}
