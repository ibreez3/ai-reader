package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ibreez3/ai-reader/novel"
	"github.com/ibreez3/ai-reader/openai"
)

func main() {
    topic := flag.String("topic", "", "小说主题")
    model := flag.String("model", "gpt-4o-mini", "模型名称")
    out := flag.String("out", "output", "输出目录")
    baseURL := flag.String("base-url", "https://api.openai.com/v1", "API Base URL")
    chapters := flag.Int("chapters", 10, "章节数量")
    words := flag.Int("words", 1500, "每章字数")
    preset := flag.String("preset", "xiyou_shuangwen", "预设风格")
    outlineFile := flag.String("outline-file", "", "使用指定的大纲JSON文件")
    instructionFile := flag.String("instruction-file", "", "章节附加指令文件")
    flag.Parse()
    if *topic == "" {
        log.Fatal("必须提供 --topic")
    }

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("缺少环境变量 OPENAI_API_KEY")
	}

	cli := openai.NewClient(apiKey, *baseURL)
    gen := novel.NewGenerator(cli)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

    spec := novel.Spec{Topic: *topic, Language: "zh", Model: *model, Chapters: *chapters, Words: *words, Preset: *preset}
    if *instructionFile != "" {
        b, e := os.ReadFile(*instructionFile)
        if e == nil { spec.Instruction = string(b) }
    }

    var outline novel.Outline
    var characters []novel.Character
    var contents []novel.ChapterContent
    var err error
    if *outlineFile != "" {
        data, e := os.ReadFile(*outlineFile)
        if e != nil { log.Fatal(e) }
        if e := json.Unmarshal(data, &outline); e != nil { log.Fatal(e) }
        spec.Topic = outline.Title
        outline, characters, contents, e = gen.GenerateFromOutline(ctx, spec, outline)
        if e != nil { log.Fatal(e) }
    } else {
        outline, characters, contents, err = gen.Generate(ctx, spec)
        if err != nil { log.Fatal(err) }
    }
	if err != nil {
		log.Fatal(err)
	}
    dir, err := novel.WriteToFiles(*out, outline, contents)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("已生成：", dir)
    fmt.Println("章节数：", len(contents))
    fmt.Println("标题：", outline.Title)
    fmt.Println("人物数：", len(characters))
}
