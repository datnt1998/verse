package channels

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	xdraw "golang.org/x/image/draw"
)

// assetFileOK: tên tệp nhận diện được phép phục vụ là uuid + loại tài sản +
// đuôi. Avatar giữ PNG của nguồn vẽ; ảnh bìa là JPEG (xem encodeBanner).
func assetFileOK(file string) bool {
	return regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}-(avatar\.png|banner\.jpg)$`).MatchString(file)
}

// AssetFile mở tệp nhận diện để phục vụ. Tên phải khớp mẫu và đang là avatar
// hoặc ảnh bìa của chính kênh; file mở dưới os.Root của thư mục dự án nên
// symlink (tệp hay thư mục cha) thoát ra ngoài project bị chặn ngay khi mở —
// không cần check realpath riêng trước.
func (s *Service) AssetPath(ctx context.Context, id, file string) (string, error) {
	if !assetFileOK(file) {
		return "", errNotFound("Không tìm thấy ảnh nhận diện.")
	}
	avatar, banner, found, err := s.store.ChannelAssetPaths(ctx, id)
	if err != nil {
		return "", err
	}
	if !found {
		return "", errNotFound("Kênh không tồn tại.")
	}
	matched := ""
	for _, rel := range []*string{avatar, banner} {
		if rel != nil && filepath.Base(*rel) == file {
			matched = *rel
		}
	}
	if matched == "" {
		return "", errNotFound("Ảnh nhận diện không còn được kênh dùng.")
	}
	return matched, nil
}

func (s *Service) Media() *media.Service { return s.media }

// AssetFile pins a local working file until Close. Authorization and channel
// ownership are checked before consulting the media manifest.
func (s *Service) AssetFile(ctx context.Context, id, file string) (io.ReadSeekCloser, string, int64, error) {
	matched, err := s.AssetPath(ctx, id, file)
	if err != nil {
		return nil, "", 0, err
	}
	if s.media != nil {
		handle, err := s.media.Acquire(ctx, matched)
		if err != nil {
			return nil, "", 0, err
		}
		f, err := os.Open(handle.Path)
		if err != nil {
			handle.Release()
			return nil, "", 0, err
		}
		return &assetHandle{File: f, handle: handle}, file, handle.Ref.Bytes, nil
	}
	root, err := os.OpenRoot(s.storage.Root())
	if err != nil {
		return nil, "", 0, errNotFound("Không tìm thấy ảnh nhận diện.")
	}
	defer root.Close()
	f, err := root.Open(matched)
	if err != nil {
		return nil, "", 0, errNotFound("Không tìm thấy ảnh nhận diện.")
	}
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		f.Close()
		return nil, "", 0, errNotFound("Không tìm thấy ảnh nhận diện.")
	}
	return f, file, st.Size(), nil
}

type assetHandle struct {
	*os.File
	handle *media.Handle
}

func (f *assetHandle) Close() error {
	defer f.handle.Release()
	return f.File.Close()
}

// moveFile chuyển tệp trong cùng ổ đĩa (rename nguyên tử); khác ổ thì chép
// sang tệp tạm rồi rename để không ai đọc phải tệp dở.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// coverScale đưa ảnh về đúng canvas wxh theo kiểu cover: crop phần giữa theo
// tỉ lệ đích rồi resize Catmull-Rom. Provider có thể trả kích thước khác
// request (Images API chỉ có vài size) — kích thước file công bố phải là
// canvas thật, không chỉ là gợi ý trong prompt.
func coverScale(src image.Image, w, h int) image.Image {
	sb := src.Bounds()
	if sb.Dx() == w && sb.Dy() == h {
		return src
	}
	ar := float64(w) / float64(h)
	var crop image.Rectangle
	if float64(sb.Dx())/float64(sb.Dy()) > ar {
		cw := int(float64(sb.Dy()) * ar)
		x0 := (sb.Dx() - cw) / 2
		crop = image.Rect(x0, 0, x0+cw, sb.Dy())
	} else {
		ch := int(float64(sb.Dx()) / ar)
		y0 := (sb.Dy() - ch) / 2
		crop = image.Rect(0, y0, sb.Dx(), y0+ch)
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, xdraw.Over, nil)
	return dst
}

// decodeImage đọc một ảnh PNG/JPEG từ đĩa.
func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("đọc ảnh nhận diện: %w", err)
	}
	return img, nil
}

// writePNGAtomic mã hoá PNG ra tệp tạm rồi rename nguyên tử lên đích.
func writePNGAtomic(img image.Image, dst string) error {
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	encErr := png.Encode(out, img)
	closeErr := out.Close()
	if encErr != nil || closeErr != nil {
		os.Remove(tmp)
		if encErr != nil {
			return fmt.Errorf("mã hoá avatar: %w", encErr)
		}
		return closeErr
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// encodeBanner mã hoá lại ảnh bìa sang JPEG thuần Go (không cần ffmpeg/cgo),
// hạ chất lượng 90 → 80 → 65 tới khi ≤ 6 MB; vẫn quá thì hỏng run.
func encodeBanner(img image.Image, dst string) error {
	// Bậc chất lượng JPEG của ảnh bìa: hạ dần tới khi ≤ 6 MB.
	for _, quality := range []int{90, 80, 65} {
		tmp := dst + ".tmp"
		out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		encErr := jpeg.Encode(out, img, &jpeg.Options{Quality: quality})
		closeErr := out.Close()
		if encErr != nil || closeErr != nil {
			os.Remove(tmp)
			if encErr != nil {
				return fmt.Errorf("mã hoá ảnh bìa: %w", encErr)
			}
			return closeErr
		}
		st, err := os.Stat(tmp)
		if err != nil {
			os.Remove(tmp)
			return err
		}
		if st.Size() <= BannerMaxBytes {
			if err := os.Rename(tmp, dst); err != nil {
				os.Remove(tmp)
				return err
			}
			return nil
		}
		os.Remove(tmp)
	}
	return &Error{Code: CodeInternal, Message: "Ảnh bìa vượt quá 6 MB sau khi nén.",
		Recovery: "Chạy lại tạo nhận diện."}
}
