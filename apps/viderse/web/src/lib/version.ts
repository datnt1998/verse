/** Phần semver của chuỗi phiên bản build (“v0.0.0-20261029-abc+dirty” → “0.0.0”).
 *  Chrome cài đặt chỉ hiện semver (artboard “Viderse 1.4.0”); chuỗi đầy đủ thuộc
 *  thẻ phiên bản trong nhật ký. Chuỗi lạ giữ nguyên. */
export function semverOf(raw: string): string {
  const m = /^v?(\d+\.\d+\.\d+)/.exec(raw)
  return m ? m[1] : raw
}
