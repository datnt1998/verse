import Link from "../../bridge/navigation";
import { ArrowLeft, Clapperboard, Monitor, type LucideIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { LinkPending } from "@/components/link-pending";
import type { EditorBack } from "./editor-back";

const PROJECT_LIST: EditorBack = { href: "/editor", label: "Về danh sách project" };

export function EditorState({ title, description, icon: Icon = Monitor, loading = false, retry, back }: { title: string; description: string; icon?: LucideIcon; loading?: boolean; retry?: () => void; back?: EditorBack }) {
  const target = back ?? PROJECT_LIST;
  return <main className="flex min-h-dvh min-w-0 flex-col items-center justify-center px-4 py-8 text-center" aria-label="Trình chỉnh sửa video">
    <div className="flex w-full max-w-md min-w-0 flex-col items-center gap-4">
      <Icon className="size-10 text-muted-foreground" aria-hidden />
      <h1 className="break-words text-xl font-semibold">{title}</h1>
      <p className="break-words text-sm text-muted-foreground" role={loading ? "status" : undefined}>{description}</p>
      {!loading && <div className="flex flex-wrap justify-center gap-2">{retry && <Button className="min-h-11 text-background" onClick={retry}>Thử lại</Button>}<Button asChild variant="outline" className="relative min-h-11 whitespace-normal"><Link href={target.href}><ArrowLeft aria-hidden />{target.label}<LinkPending /></Link></Button></div>}
    </div>
  </main>;
}

export function EditorLoading() {
  return <EditorState icon={Clapperboard} title="Đang mở video" description="Đang tải các đoạn và file của bạn…" loading />;
}
