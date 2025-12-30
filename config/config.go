package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
    Server struct {
        Port int `yaml:"port"`
        JobTimeoutMin int `yaml:"job_timeout_min"`
    } `yaml:"server"`
    OpenAI struct {
        BaseURL   string `yaml:"base_url"`
        Model     string `yaml:"model"`
        APIKeyEnv string `yaml:"api_key_env"`
        APIKey    string `yaml:"-"`
        RequestTimeoutSec int `yaml:"request_timeout_sec"`
        MaxRetries        int `yaml:"max_retries"`
        RetryBackoffMs    int `yaml:"retry_backoff_ms"`
    } `yaml:"openai"`
    Output struct {
        Dir string `yaml:"dir"`
    } `yaml:"output"`
}

func Load(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := parseYAMLConfig(&cfg, string(b)); err != nil {
		return cfg, err
	}
	envName := cfg.OpenAI.APIKeyEnv
	if envName == "" {
		envName = "AIREADER_OPENAI_APIKEY"
	}
	cfg.OpenAI.APIKey = os.Getenv(envName)
	if cfg.OpenAI.APIKey == "" {
		return cfg, fmt.Errorf("missing OpenAI API key in env %s", envName)
	}
    if cfg.Server.Port == 0 {
        cfg.Server.Port = 8080
    }
    if cfg.Server.JobTimeoutMin == 0 { cfg.Server.JobTimeoutMin = 60 }
    if cfg.Output.Dir == "" {
        cfg.Output.Dir = "output"
    }
    if cfg.OpenAI.RequestTimeoutSec == 0 { cfg.OpenAI.RequestTimeoutSec = 120 }
    if cfg.OpenAI.MaxRetries == 0 { cfg.OpenAI.MaxRetries = 3 }
    if cfg.OpenAI.RetryBackoffMs == 0 { cfg.OpenAI.RetryBackoffMs = 1500 }
    return cfg, nil
}

func parseYAMLConfig(cfg *Config, s string) error {
	scanner := bufio.NewScanner(strings.NewReader(s))
	section := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasSuffix(line, ":") && !strings.Contains(line, " ") {
			section = strings.TrimSuffix(line, ":")
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"')")
        switch section {
        case "server":
            if key == "port" {
                if p, err := strconv.Atoi(val); err == nil {
                    cfg.Server.Port = p
                }
            } else if key == "job_timeout_min" {
                if p, err := strconv.Atoi(val); err == nil { cfg.Server.JobTimeoutMin = p }
            }
        case "openai":
            switch key {
            case "base_url":
                cfg.OpenAI.BaseURL = val
            case "model":
                cfg.OpenAI.Model = val
            case "api_key_env":
                cfg.OpenAI.APIKeyEnv = val
            case "request_timeout_sec":
                if p, err := strconv.Atoi(val); err == nil { cfg.OpenAI.RequestTimeoutSec = p }
            case "max_retries":
                if p, err := strconv.Atoi(val); err == nil { cfg.OpenAI.MaxRetries = p }
            case "retry_backoff_ms":
                if p, err := strconv.Atoi(val); err == nil { cfg.OpenAI.RetryBackoffMs = p }
            }
        case "output":
            if key == "dir" {
                cfg.Output.Dir = val
            }
        }
    }
    return nil
}
