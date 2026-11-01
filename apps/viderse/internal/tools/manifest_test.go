// Test chuyển từ src/server/tools/tool-manifest.test.ts của yt-factory; phần
// manifest dùng chung (uv, ffmpeg) đã chuyển sang platform/toolchain.
package tools

import (
	"testing"

	"github.com/datnt1998/verse/platform/toolchain"
)

func TestEveryVidersePinnedAssetIsPinnedBySha256(t *testing.T) {
	for _, pin := range []toolchain.ToolPin{ChromePin(), CodexPin()} {
		for platform, assets := range pin.Assets {
			for _, a := range assets {
				if len(a.SHA256) != 64 {
					t.Fatalf("%s @ %s (%s) thiếu SHA-256", pin.Version, platform, a.Name)
				}
			}
		}
	}
}
