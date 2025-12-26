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
	} `yaml:"server"`
	OpenAI struct {
		BaseURL   string `yaml:"base_url"`
		Model     string `yaml:"model"`
		APIKeyEnv string `yaml:"api_key_env"`
		APIKey    string `yaml:"-"`
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
	if cfg.Output.Dir == "" {
		cfg.Output.Dir = "output"
	}
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
			}
		case "openai":
			switch key {
			case "base_url":
				cfg.OpenAI.BaseURL = val
			case "model":
				cfg.OpenAI.Model = val
			case "api_key_env":
				cfg.OpenAI.APIKeyEnv = val
			}
		case "output":
			if key == "dir" {
				cfg.Output.Dir = val
			}
		}
	}
	return nil
}
