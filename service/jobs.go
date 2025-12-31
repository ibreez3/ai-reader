package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ibreez3/ai-reader/config"
	"github.com/ibreez3/ai-reader/novel"
	"github.com/ibreez3/ai-reader/openai"
)

type JobStatus string

const (
	JobPending JobStatus = "pending"
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "completed"
	JobFailed  JobStatus = "failed"
)

type Job struct {
	ID        string
	Status    JobStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	Completed int
	Total     int
	Dir       string
	Error     string
	LogPath   string
	WorkDir   string
}

type Manager struct {
    mu   sync.Mutex
    jobs map[string]*Job
    chMu sync.Mutex
    chapters map[string]*ChapterTask
}

func NewManager() *Manager {
	return &Manager{jobs: map[string]*Job{}, chapters: map[string]*ChapterTask{}}
}

func (m *Manager) Get(id string) *Job {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.jobs[id]
}

func (m *Manager) LoadJobFromDisk(cfg config.Config, id string) (*Job, error) {
    base := filepath.Join(cfg.Output.Dir, "jobs", id)
    if _, err := os.Stat(base); err != nil { return nil, err }
    bOutline, err := os.ReadFile(filepath.Join(base, "outline.json"))
    if err != nil { return nil, err }
    var outline novel.Outline
    if err := json.Unmarshal(bOutline, &outline); err != nil { return nil, err }
    plansPath := filepath.Join(base, "plans.json")
    var total int
    if bPlans, err := os.ReadFile(plansPath); err == nil {
        var plans []novel.Chapter
        if e := json.Unmarshal(bPlans, &plans); e == nil { total = len(plans) }
    }
    comp := 0
    chapDir := filepath.Join(base, "chapters")
    if files, err := os.ReadDir(chapDir); err == nil {
        for _, f := range files { if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") { comp++ } }
    }
    j := &Job{ID: id, Status: JobDone, CreatedAt: time.Now(), UpdatedAt: time.Now(), Completed: comp, Total: total, WorkDir: base}
    j.Dir = filepath.Join(cfg.Output.Dir, sanitizeDirName(outline.Title))
    m.mu.Lock(); m.jobs[id] = j; m.mu.Unlock()
    return j, nil
}

func sanitizeDirName(s string) string {
    s = strings.TrimSpace(s)
    s = strings.ReplaceAll(s, " ", "_")
    s = strings.ReplaceAll(s, "/", "-")
    return s
}

type ChapterTaskStatus string

const (
	ChapterPending ChapterTaskStatus = "pending"
	ChapterRunning ChapterTaskStatus = "running"
	ChapterDone    ChapterTaskStatus = "completed"
	ChapterFailed  ChapterTaskStatus = "failed"
)

type ChapterTask struct {
	ID          string
	JobID       string
	Chapter     int
	Words       int
	Instruction string
	Status      ChapterTaskStatus
	Path        string
	Error       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (m *Manager) StartChapterTask(cfg config.Config, j *Job, chapter int, words int, instruction string) (*ChapterTask, error) {
	id := fmt.Sprintf("chap-%d", time.Now().UnixNano())
	t := &ChapterTask{ID: id, JobID: j.ID, Chapter: chapter, Words: words, Instruction: instruction, Status: ChapterPending, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	m.chMu.Lock()
	m.chapters[id] = t
	m.chMu.Unlock()
	go m.runChapterTask(cfg, j, t)
	return t, nil
}

func (m *Manager) GetChapterTask(id string) *ChapterTask {
	m.chMu.Lock()
	defer m.chMu.Unlock()
	return m.chapters[id]
}

func (m *Manager) runChapterTask(cfg config.Config, j *Job, t *ChapterTask) {
	t.Status = ChapterRunning
	t.UpdatedAt = time.Now()
	base := j.WorkDir
	if base == "" {
		base = filepath.Join(cfg.Output.Dir, "jobs", j.ID)
	}
	bOutline, err := os.ReadFile(filepath.Join(base, "outline.json"))
	if err != nil {
		t.Status = ChapterFailed
		t.Error = err.Error()
		t.UpdatedAt = time.Now()
		return
	}
	var outline novel.Outline
	if err := json.Unmarshal(bOutline, &outline); err != nil {
		t.Status = ChapterFailed
		t.Error = err.Error()
		t.UpdatedAt = time.Now()
		return
	}
	bChars, err := os.ReadFile(filepath.Join(base, "characters.json"))
	if err != nil {
		t.Status = ChapterFailed
		t.Error = err.Error()
		t.UpdatedAt = time.Now()
		return
	}
	var characters []novel.Character
	if err := json.Unmarshal(bChars, &characters); err != nil {
		t.Status = ChapterFailed
		t.Error = err.Error()
		t.UpdatedAt = time.Now()
		return
	}
	bPlans, err := os.ReadFile(filepath.Join(base, "plans.json"))
	if err != nil {
		t.Status = ChapterFailed
		t.Error = err.Error()
		t.UpdatedAt = time.Now()
		return
	}
	var plans []novel.Chapter
	if err := json.Unmarshal(bPlans, &plans); err != nil {
		t.Status = ChapterFailed
		t.Error = err.Error()
		t.UpdatedAt = time.Now()
		return
	}
	var plan novel.Chapter
	for _, p := range plans {
		if p.Index == t.Chapter {
			plan = p
			break
		}
	}
	if plan.Index == 0 {
		t.Status = ChapterFailed
		t.Error = "chapter plan not found"
		t.UpdatedAt = time.Now()
		return
	}
	prior := []novel.ChapterContent{}
	if t.Chapter > 1 {
		cd := filepath.Join(base, "chapters")
		files, _ := os.ReadDir(cd)
		for i := 1; i < t.Chapter; i++ {
			for _, f := range files {
				name := f.Name()
				if strings.HasPrefix(name, fmt.Sprintf("%02d_", i)) {
					path := filepath.Join(cd, name)
					data, _ := os.ReadFile(path)
					prior = append(prior, novel.ChapterContent{Index: i, Title: name, Content: string(data)})
					break
				}
			}
		}
	}
	cli := openai.NewClient(cfg.OpenAI.APIKey, cfg.OpenAI.BaseURL)
	gen := novel.NewGenerator(cli).WithPersistDir(base).WithFinalBaseDir(cfg.Output.Dir)
	spec := novel.Spec{Topic: outline.Title, Language: "zh", Model: cfg.OpenAI.Model, Words: t.Words, Instruction: t.Instruction}
	canon := novel.BuildCanon(spec, outline, characters, novel.Settings{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.JobTimeoutMin)*time.Minute)
	defer cancel()
	c, err := gen.GenerateChapterWithHistory(ctx, spec, canon, plan, prior)
	if err != nil {
		t.Status = ChapterFailed
		t.Error = err.Error()
		t.UpdatedAt = time.Now()
		return
	}
	name := fmt.Sprintf("%02d_%s.md", c.Index, sanitizeFileName(c.Title))
	t.Path = filepath.Join(base, "chapters", name)
	t.Status = ChapterDone
	t.UpdatedAt = time.Now()
}

func (m *Manager) Start(cfg config.Config, spec novel.Spec) (*Job, error) {
	id := fmt.Sprintf("job-%d", time.Now().UnixNano())
	j := &Job{ID: id, Status: JobPending, CreatedAt: time.Now(), UpdatedAt: time.Now(), Completed: 0, Total: spec.Chapters}
	m.mu.Lock()
	m.jobs[id] = j
	m.mu.Unlock()
	go m.runJob(cfg, spec, j)
	return j, nil
}

func (m *Manager) StartFromSource(cfg config.Config, spec novel.Spec, source string) (*Job, error) {
	id := fmt.Sprintf("job-%d", time.Now().UnixNano())
	j := &Job{ID: id, Status: JobPending, CreatedAt: time.Now(), UpdatedAt: time.Now(), Completed: 0, Total: spec.Chapters}
	m.mu.Lock()
	m.jobs[id] = j
	m.mu.Unlock()
	go m.runJobFromSource(cfg, spec, source, j)
	return j, nil
}

func (m *Manager) runJob(cfg config.Config, spec novel.Spec, j *Job) {
	j.Status = JobRunning
	j.UpdatedAt = time.Now()
	jl, err := NewJobLogger(cfg.Output.Dir, j.ID)
	if err == nil {
		j.LogPath = jl.Path()
		jl.Log("[任务开始] 生成小说任务启动")
	}
	j.WorkDir = filepath.Join(cfg.Output.Dir, "jobs", j.ID)
	_ = os.MkdirAll(j.WorkDir, 0o755)
	cli := openai.NewClient(cfg.OpenAI.APIKey, cfg.OpenAI.BaseURL)
	gen := novel.NewGenerator(cli)
	if err == nil {
		gen.WithLogger(jl.Log)
	}
	gen.WithPersistDir(j.WorkDir)
	timeoutMin := cfg.Server.JobTimeoutMin
	if timeoutMin <= 0 {
		timeoutMin = 60
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMin)*time.Minute)
	defer cancel()
	merged := mergeSpecDefaults(cfg, spec)
	if jl != nil {
		jl.Log(fmt.Sprintf("[参数] topic=%s chapters=%d words=%d model=%s preset=%s", merged.Topic, merged.Chapters, merged.Words, merged.Model, merged.Preset))
	}
	gen.WithPersistDir(j.WorkDir).WithFinalBaseDir(cfg.Output.Dir)
	gen.WithRequestPolicy(cfg.OpenAI.RequestTimeoutSec, cfg.OpenAI.MaxRetries, cfg.OpenAI.RetryBackoffMs)
	outline, characters, plans, err := gen.GenerateArtifacts(ctx, merged)
	if err != nil {
		if jl != nil {
			jl.Log(fmt.Sprintf("[任务失败] %s", err.Error()))
		}
		j.Status = JobFailed
		j.Error = err.Error()
		j.UpdatedAt = time.Now()
		return
	}
	j.Total = len(plans)
	if jl != nil {
		jl.Log(fmt.Sprintf("[大纲] 标题=%s 章节数=%d", outline.Title, j.Total))
	}
	_ = writeProgress(j.WorkDir, 0, j.Total)
	_ = characters // artifacts persisted; avoid unused warning
	if jl != nil {
		jl.Log("[产物就绪] outline.json / characters.json / plans.json")
	}
	j.Status = JobDone
	j.UpdatedAt = time.Now()
}

func writeProgress(dir string, completed, total int) error {
	if dir == "" {
		return nil
	}
	b := []byte(fmt.Sprintf("{\"completed\":%d,\"total\":%d}", completed, total))
	return os.WriteFile(filepath.Join(dir, "progress.json"), b, 0o644)
}

func (m *Manager) runJobFromSource(cfg config.Config, spec novel.Spec, source string, j *Job) {
	j.Status = JobRunning
	j.UpdatedAt = time.Now()
	jl, err := NewJobLogger(cfg.Output.Dir, j.ID)
	if err == nil {
		j.LogPath = jl.Path()
		jl.Log("[任务开始] 使用来源文本生成小说")
	}
	cli := openai.NewClient(cfg.OpenAI.APIKey, cfg.OpenAI.BaseURL)
	gen := novel.NewGenerator(cli)
	if err == nil {
		gen.WithLogger(jl.Log)
	}
	j.WorkDir = filepath.Join(cfg.Output.Dir, "jobs", j.ID)
	_ = os.MkdirAll(j.WorkDir, 0o755)
	gen.WithPersistDir(j.WorkDir)
	timeoutMin := cfg.Server.JobTimeoutMin
	if timeoutMin <= 0 {
		timeoutMin = 60
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMin)*time.Minute)
	defer cancel()
	merged := mergeSpecDefaults(cfg, spec)
	if jl != nil {
		jl.Log(fmt.Sprintf("[参数] topic=%s chapters=%d words=%d model=%s preset=%s", merged.Topic, merged.Chapters, merged.Words, merged.Model, merged.Preset))
	}
	outline, characters, plans, err := gen.GenerateArtifactsFromSource(ctx, merged, source)
	if err != nil {
		if jl != nil {
			jl.Log(fmt.Sprintf("[任务失败] %s", err.Error()))
		}
		j.Status = JobFailed
		j.Error = err.Error()
		j.UpdatedAt = time.Now()
		return
	}
	j.Total = len(plans)
	if jl != nil {
		jl.Log(fmt.Sprintf("[大纲] 标题=%s 章节数=%d", outline.Title, j.Total))
	}
	_ = writeProgress(j.WorkDir, 0, j.Total)
	_ = characters
	if jl != nil {
		jl.Log("[产物就绪] outline.json / characters.json / plans.json")
	}
	j.Status = JobDone
	j.UpdatedAt = time.Now()
}

func (m *Manager) GenerateChapter(cfg config.Config, j *Job, chapter int, words int, instruction string) (string, error) {
	base := j.WorkDir
	if base == "" {
		base = filepath.Join(cfg.Output.Dir, "jobs", j.ID)
	}
	bOutline, err := os.ReadFile(filepath.Join(base, "outline.json"))
	if err != nil {
		return "", err
	}
	var outline novel.Outline
	if err := json.Unmarshal(bOutline, &outline); err != nil {
		return "", err
	}
	bChars, err := os.ReadFile(filepath.Join(base, "characters.json"))
	if err != nil {
		return "", err
	}
	var characters []novel.Character
	if err := json.Unmarshal(bChars, &characters); err != nil {
		return "", err
	}
	bPlans, err := os.ReadFile(filepath.Join(base, "plans.json"))
	if err != nil {
		return "", err
	}
	var plans []novel.Chapter
	if err := json.Unmarshal(bPlans, &plans); err != nil {
		return "", err
	}
	var plan novel.Chapter
	for _, p := range plans {
		if p.Index == chapter {
			plan = p
			break
		}
	}
	if plan.Index == 0 {
		return "", fmt.Errorf("chapter plan not found")
	}
	prior := []novel.ChapterContent{}
	if chapter > 1 {
		cd := filepath.Join(base, "chapters")
		for i := 1; i < chapter; i++ {
			files, _ := os.ReadDir(cd)
			for _, f := range files {
				name := f.Name()
				if strings.HasPrefix(name, fmt.Sprintf("%02d_", i)) {
					path := filepath.Join(cd, name)
					data, _ := os.ReadFile(path)
					prior = append(prior, novel.ChapterContent{Index: i, Title: name, Content: string(data)})
					break
				}
			}
		}
	}
	cli := openai.NewClient(cfg.OpenAI.APIKey, cfg.OpenAI.BaseURL)
	gen := novel.NewGenerator(cli).WithLogger(func(s string) {}).WithPersistDir(base).WithFinalBaseDir(cfg.Output.Dir)
	spec := novel.Spec{Topic: outline.Title, Language: "zh", Model: cfg.OpenAI.Model, Words: words, Instruction: instruction}
	canon := novel.BuildCanon(spec, outline, characters, novel.Settings{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.JobTimeoutMin)*time.Minute)
	defer cancel()
	c, err := gen.GenerateChapterWithHistory(ctx, spec, canon, plan, prior)
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%02d_%s.md", c.Index, sanitizeFileName(c.Title))
	return filepath.Join(base, "chapters", name), nil
}

func sanitizeFileName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "-")
	return s
}

func mergeSpecDefaults(cfg config.Config, spec novel.Spec) novel.Spec {
	if spec.Model == "" {
		spec.Model = cfg.OpenAI.Model
	}
	if spec.Words <= 0 {
		spec.Words = 1500
	}
	if spec.Chapters <= 0 {
		spec.Chapters = 10
	}
	if spec.Preset == "" {
		spec.Preset = "爽文"
	}
	return spec
}
