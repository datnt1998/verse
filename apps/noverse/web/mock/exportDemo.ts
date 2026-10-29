// Chỉ tồn tại ở mock: “giơ cờ” để lần ghép kế tiếp dừng ở chương 1 vì thiếu nhạc mở đầu.
// Tách khỏi audio.ts/jobs.ts để hai tệp đó không phải import vòng nhau.
// Dùng khi đo thẻ nổi bgFail (MAudio) / toast mergeFailed (AudioExport.dc.html):
//   curl -XPOST localhost:5200/api/jobs/demo -d '{"action":"exportFail"}'
let armed: 'intro' | undefined

export const armExportFailure = () => { armed = 'intro' }

/** Đọc rồi tự xoá — mỗi lần giơ cờ chỉ hỏng đúng một lần ghép. */
export const takeExportFailure = (): 'intro' | undefined => {
  const next = armed
  armed = undefined
  return next
}
