const MAX_LUT_SIZE = 65;

/** Reject oversized LUTs before the parser allocates their cubic sample buffer. */
export function assertSupportedLutSize(source: string): void {
  const header = /^\s*LUT_3D_SIZE\s+(\d+)(?:\s|$)/m.exec(source);
  if (header && Number(header[1]) > MAX_LUT_SIZE) {
    throw new Error("Bảng màu quá lớn. Chọn file không vượt quá 65³.");
  }
}
