package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
}

func NewManager() *Manager { return &Manager{jobs: map[string]*Job{}} }

func (m *Manager) Get(id string) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobs[id]
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
    gen.WithPersistDir(j.WorkDir).WithFinalBaseDir(cfg.Output.Dir)
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
	outline, _, contents, err := gen.GenerateWithProgress(ctx, merged, func(idx int, ch novel.ChapterContent) {
		j.Completed++
		j.UpdatedAt = time.Now()
		if jl != nil {
			jl.Log(fmt.Sprintf("[章节完成] 第%d章 %s", ch.Index, ch.Title))
		}
		_ = writeProgress(j.WorkDir, j.Completed, j.Total)
	})
	if err != nil {
		if jl != nil {
			jl.Log(fmt.Sprintf("[任务失败] %s", err.Error()))
		}
		j.Status = JobFailed
		j.Error = err.Error()
		j.UpdatedAt = time.Now()
		return
	}
	j.Total = len(contents)
	if jl != nil {
		jl.Log(fmt.Sprintf("[大纲] 标题=%s 章节数=%d", outline.Title, j.Total))
	}
	_ = writeProgress(j.WorkDir, j.Completed, j.Total)
	dir, e := novel.WriteToFiles(cfg.Output.Dir, outline, contents)
	if e != nil {
		if jl != nil {
			jl.Log(fmt.Sprintf("[写入失败] %s", e.Error()))
		}
		j.Status = JobFailed
		j.Error = e.Error()
		j.UpdatedAt = time.Now()
		return
	}
	j.Dir = dir
	if jl != nil {
		jl.Log(fmt.Sprintf("[任务结束] 输出目录：%s", dir))
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	merged := mergeSpecDefaults(cfg, spec)
	if jl != nil {
		jl.Log(fmt.Sprintf("[参数] topic=%s chapters=%d words=%d model=%s preset=%s", merged.Topic, merged.Chapters, merged.Words, merged.Model, merged.Preset))
	}
	outline, _, contents, err := gen.GenerateFromSource(ctx, merged, source)
	if err != nil {
		if jl != nil {
			jl.Log(fmt.Sprintf("[任务失败] %s", err.Error()))
		}
		j.Status = JobFailed
		j.Error = err.Error()
		j.UpdatedAt = time.Now()
		return
	}
	j.Total = len(contents)
	if jl != nil {
		jl.Log(fmt.Sprintf("[大纲] 标题=%s 章节数=%d", outline.Title, j.Total))
	}
	dir, e := novel.WriteToFiles(cfg.Output.Dir, outline, contents)
	if e != nil {
		if jl != nil {
			jl.Log(fmt.Sprintf("[写入失败] %s", e.Error()))
		}
		j.Status = JobFailed
		j.Error = e.Error()
		j.UpdatedAt = time.Now()
		return
	}
	j.Dir = dir
	if jl != nil {
		jl.Log(fmt.Sprintf("[任务结束] 输出目录：%s", dir))
	}
	j.Status = JobDone
	j.UpdatedAt = time.Now()
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
