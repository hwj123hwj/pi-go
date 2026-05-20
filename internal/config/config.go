package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Name        string
	Host        string
	Port        int
	DataDir     string
	SessionFile string
	MaxTurns    int
	Timeout     time.Duration

	// Provider
	Provider string // mock / anthropic / openai

	// Anthropic
	AnthropicAPIKey  string
	AnthropicModel   string
	AnthropicBaseURL string

	// OpenAI-compatible
	OpenAIAPIKey  string
	OpenAIModel   string
	OpenAIBaseURL string

	// Tool sandbox
	Workspace          string
	EnableBash         bool
	BashTimeoutSeconds int
}

func Default() Config {
	return Config{
		Name:        "pi-go",
		Host:        "127.0.0.1",
		Port:        8080,
		DataDir:     "./data",
		SessionFile: "./data/session.jsonl",
		MaxTurns:    8,
		Timeout:     5 * time.Minute,

		Provider: "mock",

		AnthropicBaseURL: "https://api.anthropic.com",

		OpenAIBaseURL: "https://api.openai.com/v1",

		Workspace:          "/tmp/pi-go-workspace",
		EnableBash:         false,
		BashTimeoutSeconds: 30,
	}
}

// LoadFromEnv 从环境变量中加载配置，覆盖默认值。
func (c *Config) LoadFromEnv() {
	if v := os.Getenv("PI_GO_PROVIDER"); v != "" {
		c.Provider = v
	}
	if v := os.Getenv("PI_GO_HOST"); v != "" {
		c.Host = v
	}
	if v := os.Getenv("PI_GO_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Port = p
		}
	}
	if v := os.Getenv("PI_GO_SESSION_FILE"); v != "" {
		c.SessionFile = v
	}
	if v := os.Getenv("PI_GO_WORKSPACE"); v != "" {
		c.Workspace = v
	}
	if v := os.Getenv("PI_GO_ENABLE_BASH"); v != "" {
		c.EnableBash = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("PI_GO_BASH_TIMEOUT_SECONDS"); v != "" {
		if t, err := strconv.Atoi(v); err == nil {
			c.BashTimeoutSeconds = t
		}
	}

	// Anthropic
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		c.AnthropicAPIKey = v
	}
	if v := os.Getenv("ANTHROPIC_MODEL"); v != "" {
		c.AnthropicModel = v
	}
	if v := os.Getenv("ANTHROPIC_BASE_URL"); v != "" {
		c.AnthropicBaseURL = v
	}

	// OpenAI
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		c.OpenAIAPIKey = v
	}
	if v := os.Getenv("OPENAI_MODEL"); v != "" {
		c.OpenAIModel = v
	}
	if v := os.Getenv("OPENAI_BASE_URL"); v != "" {
		c.OpenAIBaseURL = v
	}
}

// LoadDotEnv 从 .env 文件读取键值对，设置到环境变量中。
// .env 文件中的值会覆盖已有的环境变量。
func LoadDotEnv(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		_ = os.Setenv(key, value)
	}
	return nil
}
