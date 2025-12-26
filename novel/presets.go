package novel

import "strings"

const PresetXiyouBase = "你是专业的玄幻爽文+西游衍生小说创作者，精通以下规则:\n1. 世界观：以西游三界（人/神/妖/魔/佛）为基础，可新增原创势力，但需符合玄幻逻辑；\n2. 境界体系：凡仙→地仙→天仙→金仙→太乙金仙→大罗金仙→准圣→圣人→天道→鸿蒙，每个境界有明确能力标签；\n3. 爽点要求：每3-5章一个小爽点，10-15章一个中爽点，30章一个大爽点，打脸情节要直接，升级节奏要快；\n4. 金手指要求：必须与西游绑定，有成长性和限制，不能过于无敌（前期需有挑战）；\n5. 语言风格：简洁有力，动作描写生动，对话符合角色性格（主角桀骜，反派嚣张，配角烘托），避免冗余描写；\n6. 原创要求：主角为原创，原著角色仅作为辅助，剧情不能复刻西游，需有新冲突；\n7. 合规要求：不涉及敏感内容，不写血腥暴力、色情低俗情节，不违背公序良俗。"

const GenericSettingBase = "你是资深小说设定与世界观构建专家。基于提供的主题或文本材料，生成结构化设定，要求逻辑自洽、风格统一、避免模板化措辞，且仅输出JSON结果。"

const PresetXiyouSetting = "请按以下结构仅输出JSON（无任何额外文本或注释）：{\"protagonist\":{\"personality\":...,\"background\":...,\"goal\":...},\"golden_finger\":{\"name\":...,\"activation\":...,\"initial\":...,\"upgrade\":...,\"limit\":...},\"world_fusion\":{\"relations\":...,\"start_location\":...,\"initial_crisis\":...},\"realms\":{\"current\":...,\"next\":[...],\"breakthrough\":{境界:条件}}}。要求：符合爽文逻辑，金手指与西游强绑定，初始危机能快速引出第一个爽点。"

const GenericSettingSchema = "请按以下结构仅输出JSON（无任何额外文本或注释）：{\"protagonist\":{\"personality\":...,\"background\":...,\"goal\":...},\"signature_elements\":{\"devices\":...,\"constraints\":...,\"progression\":...},\"world\":{\"relations\":...,\"start_location\":...,\"initial_crisis\":...}}"

func BuildSettingPrompt(preset string, topic string) (string, string) {
	sys := GenericSettingBase
	if preset == "xiyou_shuangwen" {
		sys = PresetXiyouBase
	} else if preset != "" {
		sys = preset
	}
	b := strings.Builder{}
	if topic != "" {
		b.WriteString("主题：")
		b.WriteString(topic)
		b.WriteString("\n")
	}
	if preset == "xiyou_shuangwen" {
		b.WriteString(PresetXiyouSetting)
	} else {
		b.WriteString(GenericSettingSchema)
	}
	return sys, b.String()
}

func BuildChapterInstructionPreset() string {
	return "基于设定与大纲生成章节。开头快速切入关键场景；中段推进冲突与反差爽点；结尾引出下一关键地点。语言风格动作生动、对话简洁有力，逻辑自洽。"
}

func BuildSystemFromCategories(gender string, categories, tags []string) string {
	var b strings.Builder
	base := "你是资深小说写作助手，保持自然口语化与细节真实，避免模板化措辞与机械排序。"
	b.WriteString(base)
	if gender != "" {
		if strings.ToLower(gender) == "male" {
			b.WriteString(" 男频取向，视角以男性为主，节奏爽点更直接。")
		} else if strings.ToLower(gender) == "female" {
			b.WriteString(" 女频取向，情感与关系戏份更足，细节更柔和。")
		}
	}
	if len(categories) > 0 {
		b.WriteString(" 分类要求：")
		b.WriteString(strings.Join(categories, ", "))
		b.WriteString("。")
	}
	if len(tags) > 0 {
		b.WriteString(" 标签：")
		b.WriteString(strings.Join(tags, ", "))
		b.WriteString("。")
	}
	return b.String()
}

func BuildSettingPromptWithCategories(preset, topic string, categories, tags []string) (string, string) {
	sys, user := BuildSettingPrompt(preset, topic)
	if len(categories) > 0 || len(tags) > 0 {
		u := strings.Builder{}
		u.WriteString(user)
		if len(categories) > 0 {
			u.WriteString("\n分类偏好：")
			u.WriteString(strings.Join(categories, ", "))
		}
		if len(tags) > 0 {
			u.WriteString("\n标签：")
			u.WriteString(strings.Join(tags, ", "))
		}
		user = u.String()
	}
	return sys, user
}
