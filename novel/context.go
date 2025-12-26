package novel

import (
	"strconv"
	"strings"
)

type Canon struct {
	Topic      string
	Title      string
	Language   string
	Style      string
	Characters []Character
	Settings   Settings
}

func BuildCanon(spec Spec, outline Outline, characters []Character, settings Settings) Canon {
	return Canon{
		Topic:      spec.Topic,
		Title:      outline.Title,
		Language:   spec.Language,
		Style:      "叙事连贯、语言优雅、细节真实、情节合逻辑、保持统一世界观与人物性格稳定",
		Characters: characters,
		Settings:   settings,
	}
}

func SelectRelevantCharacters(plan Chapter, characters []Character, top int) []Character {
	var picked []Character
	for _, c := range characters {
		if containsWord(plan.Title, c.Name) || containsWord(plan.Summary, c.Name) {
			picked = append(picked, c)
		}
	}
	if len(picked) == 0 {
		for i := 0; i < len(characters) && i < top; i++ {
			picked = append(picked, characters[i])
		}
	}
	return picked
}

func BuildChapterPrompt(c Canon, plan Chapter, relevant []Character, words int, extra string, system string) (string, string) {
	sys := system
	if sys == "" {
		sys = "你是资深中文小说写作助手，严格遵守风格与世界观"
	}
	b := strings.Builder{}
	b.WriteString("风格：")
	b.WriteString(c.Style)
	b.WriteString("\n标题：")
	b.WriteString(c.Title)
	b.WriteString("\n章节：")
	b.WriteString(plan.Title)
	b.WriteString("\n梗概：")
	b.WriteString(plan.Summary)
	if len(relevant) > 0 {
		b.WriteString("\n人物：\n")
		for _, r := range relevant {
			b.WriteString(r.Name)
			b.WriteString("|")
			b.WriteString(r.Role)
			b.WriteString("|")
			b.WriteString(strings.Join([]string(r.Traits), "、"))
			b.WriteString("|")
			b.WriteString(r.Background)
			b.WriteString("\n")
		}
	}
	if c.Settings.GoldenFinger.Name != "" {
		b.WriteString("\n设定：\n")
		b.WriteString("主角：")
		b.WriteString(c.Settings.Protagonist.Personality)
		b.WriteString("|")
		b.WriteString(c.Settings.Protagonist.Background)
		b.WriteString("|")
		b.WriteString(c.Settings.Protagonist.Goal)
		b.WriteString("\n金手指：")
		b.WriteString(c.Settings.GoldenFinger.Name)
		b.WriteString("|")
		b.WriteString(c.Settings.GoldenFinger.Activation)
		b.WriteString("|")
		b.WriteString(c.Settings.GoldenFinger.Initial)
		b.WriteString("|")
		b.WriteString(c.Settings.GoldenFinger.Upgrade)
		b.WriteString("|")
		b.WriteString(c.Settings.GoldenFinger.Limit)
		b.WriteString("\n世界融合：")
		b.WriteString(c.Settings.WorldFusion.Relations)
		b.WriteString("|")
		b.WriteString(c.Settings.WorldFusion.StartLocation)
		b.WriteString("|")
		b.WriteString(c.Settings.WorldFusion.InitialCrisis)
		b.WriteString("\n境界：")
		b.WriteString(c.Settings.Realms.Current)
		b.WriteString("→")
		for i, n := range c.Settings.Realms.Next {
			if i > 0 {
				b.WriteString("→")
			}
			b.WriteString(n)
		}
	}
	b.WriteString("\n要求：输出该章节完整正文，字数不少于")
	b.WriteString(strings.TrimSpace(fmtInt(words)))
	b.WriteString("字，避免与其他章节冲突与重复，保持人物设定与世界观一致")
	if extra != "" {
		b.WriteString("\n附加指令：")
		b.WriteString(extra)
	}
	b.WriteString("\n人性化要求：\n")
	b.WriteString(humanizeGuidelines())
	return sys, b.String()
}

func containsWord(s, w string) bool {
	s = strings.ToLower(s)
	w = strings.ToLower(w)
	return strings.Contains(s, w)
}

func fmtInt(i int) string {
	if i <= 0 {
		return "1200"
	}
	return strconv.FormatInt(int64(i), 10)
}

func humanizeGuidelines() string {
	var b strings.Builder
	b.WriteString("人设塑造：加入具体缺陷、反差与动机，赋予真实习惯与隐藏创伤，避免空泛形容词。示例：表面温柔实则社恐，紧张时反复摸器具；退休消防员跛行、毒舌但心软，口头禅带‘想当年’。\n")
	b.WriteString("语言风格：以短句与口语表达为主，允许逻辑跳跃与重复，不用‘首先/其次’‘不但/而且’‘综上所述’，避免‘维度’‘底层逻辑’等术语，改用大白话，可用‘额…’‘其实吧’‘也不是说’等自然过渡。\n")
	b.WriteString("情节设计：允许犹豫与两难选择，加入意外细节与不完美决定，避免善恶分明与最优解式推进，角色可明知故犯或临时变卦但逻辑自洽。\n")
	b.WriteString("细节填充：用五感与生活碎片呈现情绪，加入小BUG与情绪锚点。示例：泪水砸在屏幕上晕开记录、指尖揉皱纸巾、喉咙发紧；雨伞被风吹翻、裤脚沾泥、屏幕进水；旧照片触发阳光味洗衣粉、外婆方言、照片边缘磨损的记忆。\n")
	return b.String()
}
