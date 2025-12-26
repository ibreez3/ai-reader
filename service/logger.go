package service

import (
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "time"
)

type JobLogger struct {
    mu      sync.Mutex
    path    string
    started bool
}

func NewJobLogger(baseDir, jobID string) (*JobLogger, error) {
    logsDir := filepath.Join(baseDir, "jobs")
    if err := os.MkdirAll(logsDir, 0o755); err != nil { return nil, err }
    p := filepath.Join(logsDir, fmt.Sprintf("%s.log", jobID))
    return &JobLogger{path: p}, nil
}

func (l *JobLogger) Log(msg string) {
    l.mu.Lock()
    defer l.mu.Unlock()
    f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
    if err != nil { return }
    defer f.Close()
    ts := time.Now().Format("2006-01-02 15:04:05")
    _, _ = f.WriteString(fmt.Sprintf("[%s] %s\n", ts, msg))
}

func (l *JobLogger) Path() string { return l.path }
