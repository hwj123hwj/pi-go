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

	// DeepV Server
	DeepVEnabled   bool
	DeepVServerURL string
	DeepVModel     string
	DeepVWorkDir   string

	// Tool sandbox
	Workspace          string
	EnableBash         bool
	BashTimeoutSeconds int

	// Tool output
	MaxOutputLen int

	// Tool filtering
	AllowedTools []string
	BlockedTools []string

	// Prompt
	HistoryFile    string
	PromptTemplate string
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

		DeepVServerURL: "https://api-code.deepvlab.ai",

		Workspace:          "", // empty = use cwd
		EnableBash:         false,
		BashTimeoutSeconds: 30,

		MaxOutputLen: 30000,
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

	// DeepV Server
	if v := os.Getenv("DEEPV_ENABLED"); v != "" {
		c.DeepVEnabled = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("DEEPV_SERVER_URL"); v != "" {
		c.DeepVServerURL = v
	}
	if v := os.Getenv("DEEPV_MODEL"); v != "" {
		c.DeepVModel = v
	}
	if v := os.Getenv("DEEPV_WORK_DIR"); v != "" {
		c.DeepVWorkDir = v
	}

	// Tool output
	if v := os.Getenv("PI_GO_MAX_OUTPUT_LEN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.MaxOutputLen = n
		}
	}

	// Tool filtering
	if v := os.Getenv("PI_GO_ALLOWED_TOOLS"); v != "" {
		c.AllowedTools = strings.Split(v, ",")
	}
	if v := os.Getenv("PI_GO_BLOCKED_TOOLS"); v != "" {
		c.BlockedTools = strings.Split(v, ",")
	}

	// Prompt
	if v := os.Getenv("PI_GO_HISTORY_FILE"); v != "" {
		c.HistoryFile = v
	}
	if v := os.Getenv("PI_GO_PROMPT_TEMPLATE"); v != "" {
		c.PromptTemplate = v
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
