// Luật của Bản tin, tách khỏi màn để chunk đầu không kéo cả màn Bản tin:
// chỉ hàm thuần, dùng được ở WorkbenchScreen và ở test.

export function shouldAutoOpenRecap(from: string, to: string, hasNew: boolean, enabled: boolean): boolean {
  const start = new Date(from)
  const end = new Date(to)
  if (!hasNew || !enabled || !Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime())) return false
  const five = new Date(end)
  five.setHours(5, 0, 0, 0)
  return end.getTime() - start.getTime() >= 3 * 60 * 60 * 1000 || (end >= five && start < five)
}

export function recapLead(waiting: number, ready: number): string {
  return `Trong lúc bạn vắng: ${ready} video sẵn sàng đăng, ${waiting} video chờ bạn.`
}
