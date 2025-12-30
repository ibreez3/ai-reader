package novel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ibreez3/ai-reader/openai"
)

type Generator struct {
	Client     *openai.Client
	Log        func(string)
	PersistDir string
}

func NewGenerator(cli *openai.Client) *Generator {
	return &Generator{Client: cli}
}

func (g *Generator) WithLogger(log func(string)) *Generator {
	g.Log = log
	return g
}

func (g *Generator) WithPersistDir(dir string) *Generator {
	g.PersistDir = dir
	return g
}

func (g *Generator) Generate(ctx context.Context, spec Spec) (Outline, []Character, []ChapterContent, error) {
	outline, err := g.generateOutline(ctx, spec)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	if g.PersistDir != "" {
		_ = persistOutline(g.PersistDir, outline)
	}
	characters, err := g.generateCharacters(ctx, spec, outline)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	if g.PersistDir != "" {
		_ = persistCharacters(g.PersistDir, characters)
	}
	if g.Log != nil {
		for _, c := range characters {
			g.Log(fmt.Sprintf("[人物生成] %s | %s | %s | %s", c.Name, c.Role, strings.Join([]string(c.Traits), "、"), c.Background))
		}
	}
	plans, err := g.generateChapterPlans(ctx, spec, outline)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	if g.PersistDir != "" {
		_ = persistPlans(g.PersistDir, plans)
	}

	settings, _ := g.generateSettings(ctx, spec)
	canon := BuildCanon(spec, outline, characters, settings)
	contents, err := g.generateChapterContentsParallel(ctx, spec, canon, plans)
	if err != nil {
		return Outline{}, nil, nil, err
	}

	issues, err := g.coherenceAudit(ctx, spec, canon, contents)
	if err == nil && len(issues) > 0 {
		revised, e := g.applyCoherenceFixes(ctx, spec, canon, contents, issues)
		if e == nil && len(revised) == len(contents) {
			contents = revised
		}
	}

	return outline, characters, contents, nil
}

func (g *Generator) GenerateWithProgress(ctx context.Context, spec Spec, onChapter func(idx int, ch ChapterContent)) (Outline, []Character, []ChapterContent, error) {
	outline, err := g.generateOutline(ctx, spec)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	if g.PersistDir != "" {
		_ = persistOutline(g.PersistDir, outline)
	}
	characters, err := g.generateCharacters(ctx, spec, outline)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	if g.PersistDir != "" {
		_ = persistCharacters(g.PersistDir, characters)
	}
	if g.Log != nil {
		for _, c := range characters {
			g.Log(fmt.Sprintf("[人物生成] %s | %s | %s | %s", c.Name, c.Role, strings.Join([]string(c.Traits), "、"), c.Background))
		}
	}
	plans, err := g.generateChapterPlans(ctx, spec, outline)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	if g.PersistDir != "" {
		_ = persistPlans(g.PersistDir, plans)
	}
	settings, _ := g.generateSettings(ctx, spec)
	canon := BuildCanon(spec, outline, characters, settings)
	contents, err := g.generateChapterContentsParallelWithCallback(ctx, spec, canon, plans, func(c ChapterContent) {
		if onChapter != nil {
			onChapter(c.Index, c)
		}
		if g.PersistDir != "" {
			_ = persistChapter(g.PersistDir, outline.Title, c)
		}
	})
	if err != nil {
		return Outline{}, nil, nil, err
	}
	issues, err := g.coherenceAudit(ctx, spec, canon, contents)
	if err == nil && len(issues) > 0 {
		revised, e := g.applyCoherenceFixes(ctx, spec, canon, contents, issues)
		if e == nil && len(revised) == len(contents) {
			contents = revised
		}
	}
	return outline, characters, contents, nil
}

func (g *Generator) GenerateFromOutline(ctx context.Context, spec Spec, outline Outline) (Outline, []Character, []ChapterContent, error) {
	if g.PersistDir != "" {
		_ = persistOutline(g.PersistDir, outline)
	}
	characters, err := g.generateCharacters(ctx, spec, outline)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	if g.PersistDir != "" {
		_ = persistCharacters(g.PersistDir, characters)
	}
	if g.Log != nil {
		for _, c := range characters {
			g.Log(fmt.Sprintf("[人物生成] %s | %s | %s | %s", c.Name, c.Role, strings.Join([]string(c.Traits), "、"), c.Background))
		}
	}
	plans, err := g.generateChapterPlans(ctx, spec, outline)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	if g.PersistDir != "" {
		_ = persistPlans(g.PersistDir, plans)
	}
	settings, _ := g.generateSettings(ctx, spec)
	canon := BuildCanon(spec, outline, characters, settings)
	contents, err := g.generateChapterContentsParallel(ctx, spec, canon, plans)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	issues, err := g.coherenceAudit(ctx, spec, canon, contents)
	if err == nil && len(issues) > 0 {
		revised, e := g.applyCoherenceFixes(ctx, spec, canon, contents, issues)
		if e == nil && len(revised) == len(contents) {
			contents = revised
		}
	}
	return outline, characters, contents, nil
}

func (g *Generator) GenerateFromSource(ctx context.Context, spec Spec, source string) (Outline, []Character, []ChapterContent, error) {
	outline, err := g.parseOutlineFromText(ctx, spec, source)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	if g.PersistDir != "" {
		_ = persistOutline(g.PersistDir, outline)
	}
	characters, err := g.parseCharactersFromText(ctx, spec, source, outline)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	if g.PersistDir != "" {
		_ = persistCharacters(g.PersistDir, characters)
	}
	settings, _ := g.generateSettings(ctx, spec)
	canon := BuildCanon(spec, outline, characters, settings)
	plans, err := g.generateChapterPlans(ctx, spec, outline)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	if g.PersistDir != "" {
		_ = persistPlans(g.PersistDir, plans)
	}
	contents, err := g.generateChapterContentsParallel(ctx, spec, canon, plans)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	issues, err := g.coherenceAudit(ctx, spec, canon, contents)
	if err == nil && len(issues) > 0 {
		revised, e := g.applyCoherenceFixes(ctx, spec, canon, contents, issues)
		if e == nil && len(revised) == len(contents) {
			contents = revised
		}
	}
	return outline, characters, contents, nil
}

func (g *Generator) parseOutlineFromText(ctx context.Context, spec Spec, source string) (Outline, error) {
	sys := "你是资深小说大纲抽取专家，仅输出JSON"
	user := "从以下文本抽取小说大纲，返回JSON：{title, chapters:[{index,title,summary}]}；仅输出JSON。要求：每个chapter仅代表单独一章；index严格为单个数字，不得包含范围表达（如1-30章）；不得卷级汇总，每条仅一章。\n" + source
	out, err := g.Client.Chat(ctx, spec.Model, sys, user)
	if err != nil {
		return Outline{}, err
	}
	var outline Outline
	j := extractJSON(out)
	if e := json.Unmarshal([]byte(j), &outline); e != nil {
		if g.Log != nil {
			g.Log("[抽取大纲失败] " + e.Error())
		}
		// 强制要求代码块JSON重试
		user = "```json\n仅输出完整JSON，无额外文本。结构：{\"title\":...,\"chapters\":[{\"index\":1,\"title\":...,\"summary\":...}]}\n```\n文本：\n" + source
		out2, err2 := g.Client.Chat(ctx, spec.Model, sys, user)
		if err2 != nil {
			// 大文本分片增量抽取
			chOutline, e2 := g.extractOutlineChunked(ctx, spec, source)
			if e2 != nil {
				return Outline{}, err2
			}
			outline = chOutline
		} else {
			j2 := extractJSON(out2)
			if err := json.Unmarshal([]byte(j2), &outline); err != nil {
				// 大文本分片增量抽取
				chOutline, e2 := g.extractOutlineChunked(ctx, spec, source)
				if e2 != nil {
					return Outline{}, err
				}
				outline = chOutline
			}
		}
	}
	for i := range outline.Chapters {
		outline.Chapters[i].Index = i + 1
	}
	if spec.Chapters > 0 && len(outline.Chapters) != spec.Chapters {
		normalized, e := g.normalizeOutline(ctx, spec, outline, source)
		if e == nil && len(normalized.Chapters) == spec.Chapters {
			outline = normalized
		}
	}
	return outline, nil
}

func (g *Generator) parseCharactersFromText(ctx context.Context, spec Spec, source string, outline Outline) ([]Character, error) {
	sys := "你是资深人物设定抽取专家，仅输出JSON数组"
	b := strings.Builder{}
	b.WriteString("从以下文本抽取主要人物，返回JSON数组[{name,role,traits,background}]；仅输出JSON数组。\n")
	b.WriteString("标题：")
	b.WriteString(outline.Title)
	b.WriteString("\n文本：\n")
	b.WriteString(source)
	out, err := g.Client.Chat(ctx, spec.Model, sys, b.String())
	if err != nil {
		return nil, err
	}
	var characters []Character
	j := extractJSON(out)
	if e := json.Unmarshal([]byte(j), &characters); e != nil {
		if g.Log != nil {
			g.Log("[抽取人物失败] " + e.Error())
		}
		// 代码块JSON重试
		b2 := strings.Builder{}
		b2.WriteString("```json\n仅输出JSON数组，无额外文本。结构：[{\"name\":...,\"role\":...,\"traits\":[...],\"background\":...}]\n```\n标题：")
		b2.WriteString(outline.Title)
		b2.WriteString("\n文本：\n")
		b2.WriteString(source)
		out2, err2 := g.Client.Chat(ctx, spec.Model, sys, b2.String())
		if err2 != nil {
			// 分片增量抽取
			chChars, e2 := g.extractCharactersChunked(ctx, spec, source, outline)
			if e2 != nil {
				return nil, err2
			}
			characters = chChars
		} else {
			j2 := extractJSON(out2)
			if err := json.Unmarshal([]byte(j2), &characters); err != nil {
				// 分片增量抽取
				chChars, e2 := g.extractCharactersChunked(ctx, spec, source, outline)
				if e2 != nil {
					return nil, err
				}
				characters = chChars
			}
		}
	}
	return characters, nil
}

func (g *Generator) generateOutline(ctx context.Context, spec Spec) (Outline, error) {
	sys := "你是资深中文小说策划，输出结构化结果"
	chCount := spec.Chapters
	if chCount <= 0 {
		chCount = 10
	}
	user := fmt.Sprintf("基于主题生成小说大纲，章节数%d，返回JSON：{title, chapters:[{index,title,summary}]}; 仅输出JSON，不要任何额外说明或标注；每项仅单章，禁止范围表达（如1-30章）。主题：%s", chCount, spec.Topic)
	out, err := g.Client.Chat(ctx, spec.Model, sys, user)
	if err != nil {
		return Outline{}, err
	}
	var outline Outline
	if err := json.Unmarshal([]byte(extractJSON(out)), &outline); err != nil {
		return Outline{}, err
	}
	for i := range outline.Chapters {
		outline.Chapters[i].Index = i + 1
	}
	return outline, nil
}

func (g *Generator) generateSettings(ctx context.Context, spec Spec) (Settings, error) {
	sys, user := BuildSettingPromptWithCategories(spec.Preset, spec.Topic, spec.Categories, spec.Tags)
	out, err := g.Client.Chat(ctx, spec.Model, sys, user)
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal([]byte(extractJSON(out)), &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

func (g *Generator) generateCharacters(ctx context.Context, spec Spec, outline Outline) ([]Character, error) {
	sys := "你是资深中文小说人物设定专家，擅长写西游爽文，深谙‘低调装逼、反差碾压、爽点密集’的核心逻辑，输出结构化结果；仅输出JSON数组，无额外文本"
	buf := strings.Builder{}
	buf.WriteString("根据主题与大纲生成主要人物，返回JSON数组[{name,role,traits,background}]，仅输出JSON数组，不要任何其他文字。\n")
	buf.WriteString("主题：")
	buf.WriteString(spec.Topic)
	buf.WriteString("\n大纲标题：")
	buf.WriteString(outline.Title)
	user := buf.String()
	out, err := g.Client.Chat(ctx, spec.Model, sys, user)
	if err != nil {
		return nil, err
	}
	var characters []Character
	if err := json.Unmarshal([]byte(extractJSON(out)), &characters); err != nil {
		return nil, err
	}
	return characters, nil
}

func (g *Generator) generateChapterPlans(ctx context.Context, spec Spec, outline Outline) ([]Chapter, error) {
	sys := "你是资深中文小说剧情设计师，输出结构化结果"
	b := strings.Builder{}
	b.WriteString("根据给定大纲的每一章，扩充为更详细的章节梗概，加入3-5个关键事件。返回JSON数组：[{index,title,summary}]；仅输出JSON数组，无额外文本")
	b.WriteString("\n大纲标题：")
	b.WriteString(outline.Title)
	for _, ch := range outline.Chapters {
		b.WriteString("\n章节：")
		b.WriteString(fmt.Sprintf("%d. %s - %s", ch.Index, ch.Title, ch.Summary))
	}
	out, err := g.Client.Chat(ctx, spec.Model, sys, b.String())
	if err != nil {
		return nil, err
	}
	var plans []Chapter
	if err := json.Unmarshal([]byte(extractJSON(out)), &plans); err != nil {
		return nil, err
	}
	for i := range plans {
		plans[i].Index = i + 1
	}
	return plans, nil
}

func (g *Generator) generateChapterContentsParallel(ctx context.Context, spec Spec, canon Canon, plans []Chapter) ([]ChapterContent, error) {
	return g.generateChapterContentsParallelWithCallback(ctx, spec, canon, plans, nil)
}

func (g *Generator) generateChapterContentsParallelWithCallback(ctx context.Context, spec Spec, canon Canon, plans []Chapter, onChapter func(ChapterContent)) ([]ChapterContent, error) {
	contents := make([]ChapterContent, len(plans))
	for i := range plans {
		relevant := SelectRelevantCharacters(plans[i], canon.Characters, 3)
		if g.Log != nil {
			names := make([]string, 0, len(relevant))
			for _, rc := range relevant {
				names = append(names, rc.Name)
			}
			g.Log(fmt.Sprintf("[章节参与] 第%d章 %s | 人物：%s", plans[i].Index, plans[i].Title, strings.Join(names, ", ")))
		}
		sys, user := BuildChapterPrompt(canon, plans[i], relevant, spec.Words, spec.Instruction, spec.System)
		out, err := g.Client.Chat(ctx, spec.Model, sys, user)
		if err != nil {
			return nil, err
		}
		contents[i] = ChapterContent{Index: plans[i].Index, Title: plans[i].Title, Content: out}
		if g.PersistDir != "" {
			_ = persistChapter(g.PersistDir, canon.Title, contents[i])
		}
		if onChapter != nil {
			onChapter(contents[i])
		}
	}
	return contents, nil
}

func WriteToFiles(baseDir string, outline Outline, contents []ChapterContent) (string, error) {
	dir := filepath.Join(baseDir, safeDirName(outline.Title))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for _, c := range contents {
		fname := fmt.Sprintf("%02d_%s.md", c.Index, safeFileName(c.Title))
		path := filepath.Join(dir, fname)
		body := strings.Builder{}
		body.WriteString("# ")
		body.WriteString(c.Title)
		body.WriteString("\n\n")
		body.WriteString(c.Content)
		if err := os.WriteFile(path, []byte(body.String()), 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}

type CoherenceIssue struct {
	Chapter int    `json:"chapter"`
	Type    string `json:"type"`
	Detail  string `json:"detail"`
	FixHint string `json:"fix_hint"`
}

func (g *Generator) coherenceAudit(ctx context.Context, spec Spec, canon Canon, contents []ChapterContent) ([]CoherenceIssue, error) {
	b := strings.Builder{}
	b.WriteString("检查以下章节是否与风格、人物与世界观一致，返回JSON问题列表[{chapter,type,detail,fix_hint}]。\n")
	b.WriteString("风格：")
	b.WriteString(canon.Style)
	b.WriteString("\n人物：\n")
	for _, c := range canon.Characters {
		b.WriteString(c.Name)
		b.WriteString("|")
		b.WriteString(c.Role)
		b.WriteString("|")
		b.WriteString(strings.Join([]string(c.Traits), "、"))
		b.WriteString("|")
		b.WriteString(c.Background)
		b.WriteString("\n")
	}
	for _, ch := range contents {
		b.WriteString("\n章节\n")
		b.WriteString(fmt.Sprintf("%d %s\n", ch.Index, ch.Title))
		b.WriteString(ch.Content)
		b.WriteString("\n")
	}
	sys := "你是严苛的一致性审查员"
	out, err := g.Client.Chat(ctx, spec.Model, sys, b.String())
	if err != nil {
		return nil, err
	}
	var issues []CoherenceIssue
	if err := json.Unmarshal([]byte(extractJSON(out)), &issues); err != nil {
		return nil, err
	}
	return issues, nil
}

func (g *Generator) applyCoherenceFixes(ctx context.Context, spec Spec, canon Canon, contents []ChapterContent, issues []CoherenceIssue) ([]ChapterContent, error) {
	byChapter := map[int][]CoherenceIssue{}
	for _, is := range issues {
		byChapter[is.Chapter] = append(byChapter[is.Chapter], is)
	}
	revised := make([]ChapterContent, len(contents))
	for i := range contents {
		sys := "你是资深中文小说修订助手"
		b := strings.Builder{}
		b.WriteString("根据问题修订章节内容，保持风格一致并避免新增冲突，只返回修订后的完整正文。\n")
		b.WriteString("风格：")
		b.WriteString(canon.Style)
		b.WriteString("\n章节：")
		b.WriteString(contents[i].Title)
		b.WriteString("\n原文：\n")
		b.WriteString(contents[i].Content)
		if list, ok := byChapter[contents[i].Index]; ok && len(list) > 0 {
			b.WriteString("\n问题：\n")
			for _, is := range list {
				b.WriteString(is.Type)
				b.WriteString(":")
				b.WriteString(is.Detail)
				if is.FixHint != "" {
					b.WriteString("|")
					b.WriteString(is.FixHint)
				}
				b.WriteString("\n")
			}
		}
		out, err := g.Client.Chat(ctx, spec.Model, sys, b.String())
		if err != nil {
			return nil, err
		}
		revised[i] = ChapterContent{Index: contents[i].Index, Title: contents[i].Title, Content: out}
	}
	return revised, nil
}

func extractJSON(s string) string {
	if i := strings.Index(s, "```"); i != -1 {
		j := strings.Index(s[i+3:], "```")
		if j != -1 {
			content := s[i+3 : i+3+j]
			if nl := strings.IndexByte(content, '\n'); nl != -1 && nl < 16 {
				header := content[:nl]
				if !strings.ContainsAny(header, "{}[]") {
					content = content[nl+1:]
				}
			}
			s = content
		}
	}
	start := -1
	for idx, r := range s {
		if r == '{' || r == '[' {
			start = idx
			break
		}
	}
	if start == -1 {
		return s
	}
	depth := 0
	inStr := false
	esc := false
	end := -1
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}
	if end != -1 && end > start {
		return s[start:end]
	}
	// 回退：尝试找到最后的闭合括号，截断到该位置
	lastCloseObj := strings.LastIndexByte(s, '}')
	lastCloseArr := strings.LastIndexByte(s, ']')
	last := lastCloseObj
	if lastCloseArr > last {
		last = lastCloseArr
	}
	if last > start {
		return s[start : last+1]
	}
	return s[start:]
}

// -------- 大文本分片支持 --------

func (g *Generator) extractOutlineChunked(ctx context.Context, spec Spec, source string) (Outline, error) {
	chunks := chunkTextByParagraph(source, 8000)
	type frag struct {
		Title   string `json:"title"`
		Summary string `json:"summary"`
	}
	var all []frag
	title := spec.Topic
	for i, c := range chunks {
		sys := "你是资深小说大纲拆解专家，仅输出JSON数组"
		user := "将以下文本片段拆解为逐章列表，返回JSON数组：[{title,summary}]；仅输出JSON数组。要求：每项仅代表单独一章，不得卷级汇总或范围表达（如1-30章）。\n片段：\n" + c
		out, err := g.Client.Chat(ctx, spec.Model, sys, user)
		if err != nil {
			return Outline{}, err
		}
		var fr []frag
		if e := json.Unmarshal([]byte(extractJSON(out)), &fr); e != nil {
			if g.Log != nil {
				g.Log(fmt.Sprintf("[分片大纲失败] chunk=%d err=%s", i, e.Error()))
			}
			continue
		}
		if i == 0 && title == "" {
			// 从首片段尝试提取标题
			if len(fr) > 0 && fr[0].Title != "" {
				title = fr[0].Title
			}
		}
		all = append(all, fr...)
	}
	outline := Outline{Title: title}
	for i, f := range all {
		outline.Chapters = append(outline.Chapters, Chapter{Index: i + 1, Title: f.Title, Summary: f.Summary})
	}
	if len(outline.Chapters) == 0 {
		return Outline{}, fmt.Errorf("无法从大文本抽取大纲")
	}
	if spec.Chapters > 0 && len(outline.Chapters) != spec.Chapters {
		normalized, e := g.normalizeOutline(ctx, spec, outline, source)
		if e == nil && len(normalized.Chapters) == spec.Chapters {
			return normalized, nil
		}
	}
	return outline, nil
}

func (g *Generator) normalizeOutline(ctx context.Context, spec Spec, current Outline, source string) (Outline, error) {
	sys := "你是资深大纲拆解与扩展专家，仅输出JSON"
	b := strings.Builder{}
	b.WriteString("将以下材料与现有大纲统一，扩展为精确 ")
	b.WriteString(fmt.Sprintf("%d", spec.Chapters))
	b.WriteString(" 章，严格输出：{title, chapters:[{index,title,summary}]}。要求：index 从1到")
	b.WriteString(fmt.Sprintf("%d", spec.Chapters))
	b.WriteString("，每项仅单章；禁止范围表达如‘1-30章’；不得合并多章至一项；仅输出JSON。\n材料：\n")
	b.WriteString(source)
	b.WriteString("\n现有大纲JSON：\n")
	curJSON, _ := json.Marshal(current)
	b.Write(curJSON)
	out, err := g.Client.Chat(ctx, spec.Model, sys, b.String())
	if err != nil {
		return Outline{}, err
	}
	var outline Outline
	if e := json.Unmarshal([]byte(extractJSON(out)), &outline); e != nil {
		return Outline{}, e
	}
	for i := range outline.Chapters {
		outline.Chapters[i].Index = i + 1
	}
	return outline, nil
}

func (g *Generator) extractCharactersChunked(ctx context.Context, spec Spec, source string, outline Outline) ([]Character, error) {
	chunks := chunkTextByParagraph(source, 8000)
	dedup := map[string]Character{}
	for i, c := range chunks {
		sys := "你是资深人物设定抽取专家，仅输出JSON数组"
		b := strings.Builder{}
		b.WriteString("从以下文本片段抽取主要人物，返回JSON数组[{name,role,traits,background}]；仅输出JSON数组。\n标题：")
		b.WriteString(outline.Title)
		b.WriteString("\n片段：\n")
		b.WriteString(c)
		out, err := g.Client.Chat(ctx, spec.Model, sys, b.String())
		if err != nil {
			return nil, err
		}
		var chars []Character
		if e := json.Unmarshal([]byte(extractJSON(out)), &chars); e != nil {
			if g.Log != nil {
				g.Log(fmt.Sprintf("[分片人物失败] chunk=%d err=%s", i, e.Error()))
			}
			continue
		}
		for _, ch := range chars {
			if ch.Name == "" {
				continue
			}
			if old, ok := dedup[ch.Name]; ok {
				// 合并traits与背景
				merged := mergeCharacters(old, ch)
				dedup[ch.Name] = merged
			} else {
				dedup[ch.Name] = ch
			}
		}
	}
	res := make([]Character, 0, len(dedup))
	for _, v := range dedup {
		res = append(res, v)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("无法从大文本抽取人物")
	}
	return res, nil
}

func mergeCharacters(a, b Character) Character {
	// 合并traits
	mm := map[string]struct{}{}
	var t []string
	for _, x := range []string(a.Traits) {
		if _, ok := mm[x]; !ok && x != "" {
			mm[x] = struct{}{}
			t = append(t, x)
		}
	}
	for _, x := range []string(b.Traits) {
		if _, ok := mm[x]; !ok && x != "" {
			mm[x] = struct{}{}
			t = append(t, x)
		}
	}
	a.Traits = StringList(t)
	// 背景与角色以更长的为准
	if len(b.Background) > len(a.Background) {
		a.Background = b.Background
	}
	if len(b.Role) > len(a.Role) {
		a.Role = b.Role
	}
	return a
}

func chunkTextByParagraph(s string, max int) []string {
	if max <= 0 {
		max = 8000
	}
	parts := strings.Split(s, "\n\n")
	var chunks []string
	cur := strings.Builder{}
	for _, p := range parts {
		if cur.Len()+len(p)+2 > max {
			if cur.Len() > 0 {
				chunks = append(chunks, cur.String())
				cur.Reset()
			}
		}
		if cur.Len() > 0 {
			cur.WriteString("\n\n")
		}
		cur.WriteString(p)
	}
	if cur.Len() > 0 {
		chunks = append(chunks, cur.String())
	}
	// 若仍为空，直接按硬分割
	if len(chunks) == 0 && len(s) > 0 {
		for i := 0; i < len(s); i += max {
			end := i + max
			if end > len(s) {
				end = len(s)
			}
			chunks = append(chunks, s[i:end])
		}
	}
	return chunks
}

func persistOutline(dir string, outline Outline) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(outline, "", "  ")
	return os.WriteFile(filepath.Join(dir, "outline.json"), b, 0o644)
}

func persistCharacters(dir string, characters []Character) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(characters, "", "  ")
	return os.WriteFile(filepath.Join(dir, "characters.json"), b, 0o644)
}

func persistPlans(dir string, plans []Chapter) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(plans, "", "  ")
	return os.WriteFile(filepath.Join(dir, "plans.json"), b, 0o644)
}

func persistChapter(dir string, title string, c ChapterContent) error {
	if dir == "" {
		return nil
	}
	chapDir := filepath.Join(dir, "chapters")
	if err := os.MkdirAll(chapDir, 0o755); err != nil {
		return err
	}
	fname := fmt.Sprintf("%02d_%s.md", c.Index, safeFileName(c.Title))
	path := filepath.Join(chapDir, fname)
	body := strings.Builder{}
	body.WriteString("# ")
	body.WriteString(c.Title)
	body.WriteString("\n\n")
	body.WriteString(c.Content)
	return os.WriteFile(path, []byte(body.String()), 0o644)
}

func safeFileName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "-")
	return s
}

func safeDirName(s string) string {
	return safeFileName(s)
}
