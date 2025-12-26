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
	Client *openai.Client
	Log    func(string)
}

func NewGenerator(cli *openai.Client) *Generator {
	return &Generator{Client: cli}
}

func (g *Generator) WithLogger(log func(string)) *Generator {
	g.Log = log
	return g
}

func (g *Generator) Generate(ctx context.Context, spec Spec) (Outline, []Character, []ChapterContent, error) {
	outline, err := g.generateOutline(ctx, spec)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	characters, err := g.generateCharacters(ctx, spec, outline)
	if err != nil {
		return Outline{}, nil, nil, err
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
	characters, err := g.generateCharacters(ctx, spec, outline)
	if err != nil {
		return Outline{}, nil, nil, err
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
	settings, _ := g.generateSettings(ctx, spec)
	canon := BuildCanon(spec, outline, characters, settings)
	contents, err := g.generateChapterContentsParallelWithCallback(ctx, spec, canon, plans, func(c ChapterContent) {
		if onChapter != nil {
			onChapter(c.Index, c)
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
	characters, err := g.generateCharacters(ctx, spec, outline)
	if err != nil {
		return Outline{}, nil, nil, err
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
	characters, err := g.parseCharactersFromText(ctx, spec, source, outline)
	if err != nil {
		return Outline{}, nil, nil, err
	}
	settings, _ := g.generateSettings(ctx, spec)
	canon := BuildCanon(spec, outline, characters, settings)
	plans, err := g.generateChapterPlans(ctx, spec, outline)
	if err != nil {
		return Outline{}, nil, nil, err
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
	user := "从以下文本抽取小说大纲，返回JSON：{title, chapters:[{index,title,summary}]}；仅输出JSON。\n" + source
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
	if err := json.Unmarshal([]byte(extractJSON(out)), &characters); err != nil {
		return nil, err
	}
	return characters, nil
}

func (g *Generator) generateOutline(ctx context.Context, spec Spec) (Outline, error) {
	sys := "你是资深中文小说策划，输出结构化结果"
	chCount := spec.Chapters
	if chCount <= 0 {
		chCount = 10
	}
	user := fmt.Sprintf("基于主题生成小说大纲，章节数%d，返回JSON：{title, chapters:[{index,title,summary}]}; 仅输出JSON，不要任何额外说明或标注。主题：%s", chCount, spec.Topic)
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
	return s[start:]
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
