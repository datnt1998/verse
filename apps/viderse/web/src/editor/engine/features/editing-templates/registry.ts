import type { EditingTemplate } from './types'
import { buildFacelessCommands } from './resolver'

const templates: readonly EditingTemplate[] = [
  { id: 'narration-ken-burns-captions', name: 'Lời dẫn và ảnh chuyển động', description: 'Video dài với lời dẫn, ảnh Ken Burns và phụ đề karaoke theo từ.', category: 'narration', resolve: (context) => buildFacelessCommands(context, 'narration') },
  { id: 'listicle-sections', name: 'Danh sách theo phân đoạn', description: 'Mỗi mục có thẻ tiêu đề đánh số, hình ảnh chuyển động và phụ đề.', category: 'listicle', resolve: (context) => buildFacelessCommands(context, 'listicle') },
  { id: 'documentary-lower-thirds', name: 'Tư liệu và chú thích', description: 'Video tư liệu dài với chú thích lower-third, lời dẫn và phụ đề.', category: 'documentary', resolve: (context) => buildFacelessCommands(context, 'documentary') },
]
const byId: Record<string, EditingTemplate> = Object.fromEntries(templates.map((template) => [template.id, template]))

export function getBuiltInEditingTemplates(): readonly EditingTemplate[] { return templates }
export function getBuiltInEditingTemplate(id: string): EditingTemplate | undefined { return byId[id] }
export function getBuiltInEditingTemplatesByCategory(category: EditingTemplate['category']): readonly EditingTemplate[] { return templates.filter((template) => template.category === category) }
