// Poll tuần tự cho các luồng đăng nhập (§24.1 + §15): mỗi nhịp là một
// setTimeout SAU khi request trước xong — không bao giờ hai request chồng
// nhau; signal abort huỷ cả timer lẫn request đang bay; expired() cho phép
// UI tự kết thúc khi hết giờ dù poll còn lỗi mạng.

/** Chạy `tick` tuần tự mỗi `ms` cho tới khi tick trả false/throw/abort.
 *  Trả cleanup (gọi khi đóng hộp) + signal phát cho mọi request. */
export function sequentialPoll(ms: number, tick: (signal: AbortSignal) => Promise<boolean | void>): () => void {
  const controller = new AbortController()
  let timer: ReturnType<typeof setTimeout> | undefined
  let stopped = false

  const stop = () => {
    stopped = true
    clearTimeout(timer)
    controller.abort()
  }

  const run = async () => {
    if (stopped) return
    try {
      const keep = await tick(controller.signal)
      if (keep === false) {
        stopped = true
        return
      }
    } catch (error) {
      // Lỗi mạng/huỷ giữa chừng: abort thì dừng hẳn, còn lại tick tự quyết
      // (tick nên nuốt lỗi tạm và trả true); lỗi bất ngờ cũng dừng để tránh loop.
      if (stopped || controller.signal.aborted) return
      if ((error as { name?: string })?.name !== 'AbortError') {
        stopped = true
        return
      }
      return
    }
    if (!stopped) timer = setTimeout(() => void run(), ms)
  }

  void run()
  return stop
}
