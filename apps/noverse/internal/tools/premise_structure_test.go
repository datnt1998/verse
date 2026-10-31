package tools

import (
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// premise 的旧数据是中文标题，解析必须照旧成功，但归一名改成越南语。
func TestParsePremiseSectionsLegacyChineseHeadings(t *testing.T) {
	premise := `# Premise

## 题材和基调
东方玄幻，冷硬成长。

## 题材定位
东方玄幻升级流，面向追求爽点和关系推进的读者。

## 核心冲突
主角必须在宗门规则与个人良知之间做选择。

## 中期转向
旧有修炼路线失效，必须转向禁术体系。
`

	sections := ParsePremiseSections(premise)
	if sections["Thể loại và giọng điệu"] == "" {
		t.Fatalf("expected Thể loại và giọng điệu section, got %+v", sections)
	}
	if sections["Định vị thể loại"] == "" {
		t.Fatalf("expected Định vị thể loại section, got %+v", sections)
	}
	if sections["Xung đột cốt lõi"] == "" {
		t.Fatalf("expected Xung đột cốt lõi section, got %+v", sections)
	}
	if sections["Chuyển hướng trung kỳ"] == "" {
		t.Fatalf("expected legacy 中期转向 normalized to Chuyển hướng trung kỳ, got %+v", sections)
	}
}

func TestParsePremiseSectionsVietnameseHeadings(t *testing.T) {
	premise := `# Tiền đề cốt truyện

## Thể loại và giọng điệu
Huyền huyễn phương Đông, tông lạnh.

## Định vị thể loại
Dòng tu luyện thăng cấp, hướng tới độc giả thích khoái cảm và đẩy quan hệ.

## Xung đột cốt lõi
Nhân vật chính phải chọn giữa luật tông môn và lương tri.
`

	sections := ParsePremiseSections(premise)
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %+v", sections)
	}
	if sections["Định vị thể loại"] == "" || sections["Xung đột cốt lõi"] == "" {
		t.Fatalf("expected Vietnamese headings parsed, got %+v", sections)
	}
}

func TestParsePremiseSectionsPreservesCustomSections(t *testing.T) {
	sections := ParsePremiseSections("# Tiền đề\n## Xung đột cốt lõi\nGiữ lời hứa.\n## Phong tục địa phương\nĐổi tên vào mùa mưa.\n### Chi tiết\nTên cũ được giữ kín.\n")
	if sections["Xung đột cốt lõi"] != "Giữ lời hứa." {
		t.Fatalf("custom section leaked into canonical body: %+v", sections)
	}
	if sections["Phong tục địa phương"] != "Đổi tên vào mùa mưa.\n### Chi tiết\nTên cũ được giữ kín." || len(sections) != 2 {
		t.Fatalf("custom section content was lost: %+v", sections)
	}
}

func TestPremiseStructure(t *testing.T) {
	premise := `## Thể loại và giọng điệu
升级流，偏冷硬。

## Định vị thể loại
升级流

## Xung đột cốt lõi
冲突

## Mục tiêu nhân vật chính
目标

## Hướng kết cục
终局

## Vùng cấm sáng tác
禁区

## Điểm bán hàng khác biệt
卖点

## Móc câu khác biệt
钩子

## Cam kết cốt lõi
兑现

## Động cơ câu chuyện
引擎

## Chuyển hướng trung kỳ
转折
`

	structure := premiseStructure(premise, domain.PlanningTierMid)
	if ready, _ := structure["template_ready"].(bool); !ready {
		t.Fatalf("expected template_ready, got %+v", structure)
	}
	missing, _ := structure["missing"].([]string)
	if len(missing) != 0 {
		t.Fatalf("expected no missing headings, got %+v", missing)
	}
}

// 旧中文标题的 premise 仍要判定为结构完整，missing 用越南语规范名回给模型。
func TestPremiseStructureAcceptsLegacyChineseHeadings(t *testing.T) {
	premise := `## 题材和基调
升级流，偏冷硬。

## 题材定位
升级流

## 核心冲突
冲突

## 主角目标
目标

## 终局方向
终局

## 写作禁区
禁区

## 差异化卖点
卖点

## 差异化钩子
钩子

## 核心兑现承诺
兑现

## 故事引擎
引擎

## 中段转折
转折
`

	structure := premiseStructure(premise, domain.PlanningTierMid)
	if ready, _ := structure["template_ready"].(bool); !ready {
		t.Fatalf("expected legacy Chinese template_ready, got %+v", structure)
	}
}

func TestPremiseStructureReportsMissingInVietnamese(t *testing.T) {
	structure := premiseStructure("## Thể loại và giọng điệu\n单卷。\n", domain.PlanningTierShort)
	if ready, _ := structure["template_ready"].(bool); ready {
		t.Fatalf("expected template not ready, got %+v", structure)
	}
	missing, _ := structure["missing"].([]string)
	if len(missing) == 0 {
		t.Fatalf("expected missing headings, got %+v", structure)
	}
	for _, heading := range missing {
		if heading != "Định vị thể loại" && heading != "Tính phù hợp với truyện ngắn" {
			continue
		}
		return
	}
	t.Fatalf("expected Vietnamese canonical heading in missing, got %+v", missing)
}

func TestPremiseStructureShortAcceptsLegacyHeadingAlias(t *testing.T) {
	premise := `## 题材和基调
单卷高压营救。

## 题材定位
短篇高密度冒险。

## 核心冲突
主角必须在一夜内救出人质。

## 主角目标
救出人质并活着离开。

## 结局方向
完成任务但付出代价。

## 写作禁区
不扩展成长期连载。

## 差异化卖点
时限压力与连续反转。

## 差异化钩子
每次选择都缩短救援时间。

## 核心兑现承诺
紧迫感、抉择与反转。

## 本作为什么适合短篇/单卷收束
核心矛盾和人物弧线都能在单次任务中完成。
`

	structure := premiseStructure(premise, domain.PlanningTierShort)
	if ready, _ := structure["template_ready"].(bool); !ready {
		t.Fatalf("expected short template_ready, got %+v", structure)
	}
}
