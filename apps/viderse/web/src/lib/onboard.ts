// Trạng thái "lần đầu mở" của mảng kênh (W1 → W2). Máy cài xong mà chưa có
// kênh nào thì Tạo kênh mở trong khung riêng của Onboard (không nav), đúng
// artboard Onboard · step=create. Rời khung bằng Huỷ / Mở kênh / Khám phá thì
// ghi cờ phiên để lần sau không bị đưa lại khung đó (không lặp người dùng).
const ONBOARD_DONE = 'viderse:onboard-done'
const LAST_CREATED = 'viderse:last-created-channel'

const read = (key: string): string | null => {
  try { return sessionStorage.getItem(key) } catch { return null }
}
const write = (key: string, value: string | null) => {
  try {
    if (value === null) sessionStorage.removeItem(key)
    else sessionStorage.setItem(key, value)
  } catch { /* phiên không có storage: khung vẫn chạy theo luật kênh trống */ }
}

export const onboardDismissed = (): boolean => read(ONBOARD_DONE) === '1'
export const dismissOnboard = () => write(ONBOARD_DONE, '1')

/** Id kênh vừa tạo — bước Xong đọc lại để hiện đúng nhận diện và việc tiếp theo. */
export const setLastCreatedChannel = (id: string | null) => write(LAST_CREATED, id)
export const lastCreatedChannel = (): string | null => read(LAST_CREATED)
