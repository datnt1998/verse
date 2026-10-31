package blobstore

import (
	"fmt"
	"os"
	"path/filepath"
)

// OpenLocalExisting opens a confined local store without creating directories.
// Maintenance previews use it so an absent old root is never recreated.
func OpenLocalExisting(root string) (*Local, error) {
	if root == "" || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("BLOB_LOCAL_ROOT phải là đường dẫn tuyệt đối: %w", ErrInvalidKey)
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("mở thư mục blob đã có: %w", err)
	}
	return &Local{root: handle, path: root}, nil
}
