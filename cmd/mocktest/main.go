package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ibreez3/ai-reader/novel"
)

type MockClient struct{}

func (m *MockClient) Chat(ctx context.Context, model, system, user string) (string, error) {
	return m.respond(system, user)
}

func (m *MockClient) ChatWithRetry(ctx context.Context, model, system, user string, retries int, backoff time.Duration) (string, error) {
	return m.respond(system, user)
}

func (m *MockClient) respond(system, user string) (string, error) {
	s := strings.ToLower(system + "\n" + user)
	if strings.Contains(s, "人物") && strings.Contains(s, "[{name,role,traits,background}]") {
		chars := []novel.Character{
			{Name: "陈巽", Role: "主角", Traits: novel.StringList{"冷静", "理智"}, Background: "法医转风水师"},
			{Name: "苏晚晴", Role: "女主", Traits: novel.StringList{"干练"}, Background: "刑警队长"},
		}
		b, _ := json.Marshal(chars)
		return string(b), nil
	}
	if strings.Contains(s, "剧情设计师") || strings.Contains(s, "关键事件") {
		plans := []novel.Chapter{}
		for i := 1; i <= 5; i++ {
			plans = append(plans, novel.Chapter{Index: i, Title: fmt.Sprintf("第%d章", i), Summary: fmt.Sprintf("第%d章扩展梗概", i)})
		}
		b, _ := json.Marshal(plans)
		return string(b), nil
	}
	if strings.Contains(s, "返回json：{title, chapters") || strings.Contains(s, "仅输出完整json") || strings.Contains(s, "大纲抽取") || strings.Contains(s, "小说策划") {
		// outline with 5 chapters
		outline := struct {
			Title    string          `json:"title"`
			Chapters []novel.Chapter `json:"chapters"`
		}{Title: "测试作品"}
		for i := 1; i <= 5; i++ {
			outline.Chapters = append(outline.Chapters, novel.Chapter{Index: i, Title: fmt.Sprintf("第%d章", i), Summary: fmt.Sprintf("第%d章梗概", i)})
		}
		b, _ := json.Marshal(outline)
		return string(b), nil
	}
	// chapter content
	return "这是章节正文示例，包含若干段落与细节。", nil
}

func main() {
	cli := &MockClient{}
	gen := novel.NewGenerator(cli).WithPersistDir(filepath.Join("output", "jobs", "mock-run"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	spec := novel.Spec{Topic: "测试作品", Language: "zh", Model: "mock", Chapters: 5, Words: 100}
	outline, characters, contents, err := gen.GenerateWithProgress(ctx, spec, func(idx int, ch novel.ChapterContent) {})
	if err != nil {
		fmt.Println("生成失败:", err)
		os.Exit(1)
	}
	fmt.Println("标题:", outline.Title, "章数:", len(outline.Chapters), "人物:", len(characters), "章节:", len(contents))
	// verify files exist
	base := filepath.Join("output", "jobs", "mock-run")
	for _, p := range []string{"outline.json", "characters.json", "plans.json"} {
		if _, e := os.Stat(filepath.Join(base, p)); e != nil {
			fmt.Println("缺少文件:", p)
			os.Exit(2)
		}
	}
	if fi, e := os.ReadDir(filepath.Join(base, "chapters")); e != nil || len(fi) == 0 {
		fmt.Println("章节文件缺失")
		os.Exit(3)
	}
	fmt.Println("持久化验证通过:", base)
}
