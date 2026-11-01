package vieneu

import "github.com/datnt1998/verse/platform/toolchain"

// sourcePin là tarball mã nguồn VieNeu-TTS đúng tag đã ghim. GitHub codeload
// đóng gói lại khi kho đổi (rất hiếm); lúc đó hash lệch là lỗi cứng và bước
// tải quay về `git clone --depth 1 --branch <tag>` như trước — không bao giờ
// chấp nhận một tarball khác hash.
var sourcePin = toolchain.PinnedAsset{
	Name:      "vieneu-" + pinnedTag + ".tar.gz",
	URL:       "https://codeload.github.com/pnnbao97/VieNeu-TTS/tar.gz/refs/tags/" + pinnedTag,
	SHA256:    "06c2ebe9291fe2f9b288d99ce5bf79c954427e322d2075386ae268d3217edac6",
	SizeBytes: 3020009,
	Kind:      toolchain.AssetTarGZ,
}
