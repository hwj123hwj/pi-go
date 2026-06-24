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
	Provider string // anthropic / openai

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
	EnableWeb          bool
	WebTimeoutSeconds  int

	// Execution backend
	ExecutionMode string // "local" (default) or "ssh"
	SSHHost       string // user@host for SSH mode
	SSHPort       int    // SSH port (default 22)
	SSHWorkDir    string // Remote working directory for SSH mode

	// Tool output
	MaxOutputLen int

	// Tool filtering
	AllowedTools []string
	BlockedTools []string

	// Prompt
	HistoryFile    string
	PromptTemplate string

	// Music
	MusicPort int // Port for the music-agent server (default 8081)
}

func Default() Config {
	return Config{
		Name:        "pi-go",
		Host:        "127.0.0.1",
		Port:        8080,
		DataDir:     "./data",
		SessionFile: "./data/session.jsonl",
		MaxTurns:    200,
		Timeout:     5 * time.Minute,

		Provider: "",

		AnthropicBaseURL: "https://api.anthropic.com",

		OpenAIAPIKey:  "sk-local-gateway-hwj123hwj",
		OpenAIBaseURL: "http://localhost:4001",
		OpenAIModel:   "longcat-opus",

		Workspace:          "", // empty = use cwd
		EnableBash:         false,
		BashTimeoutSeconds: 30,
		EnableWeb:          false,
		WebTimeoutSeconds:  30,

		MaxOutputLen: 30000,

		MusicPort: 8081,
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
	if v := os.Getenv("PI_GO_DATA_DIR"); v != "" {
		c.DataDir = v
		// If SessionFile not explicitly set, derive from DataDir
		if os.Getenv("PI_GO_SESSION_FILE") == "" {
			c.SessionFile = v + "/session.jsonl"
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
	if v := os.Getenv("PI_GO_ENABLE_WEB"); v != "" {
		c.EnableWeb = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("PI_GO_WEB_TIMEOUT_SECONDS"); v != "" {
		if t, err := strconv.Atoi(v); err == nil {
			c.WebTimeoutSeconds = t
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

	// OpenAI-compatible gateway
	// PI_GO_API_KEY is the preferred name; OPENAI_API_KEY is also accepted (fallback)
	if v := os.Getenv("PI_GO_API_KEY"); v != "" {
		c.OpenAIAPIKey = v
	} else if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		c.OpenAIAPIKey = v
	}
	if v := os.Getenv("PI_GO_MODEL"); v != "" {
		c.OpenAIModel = v
	} else if v := os.Getenv("OPENAI_MODEL"); v != "" {
		c.OpenAIModel = v
	}
	if v := os.Getenv("PI_GO_BASE_URL"); v != "" {
		c.OpenAIBaseURL = v
	} else if v := os.Getenv("OPENAI_BASE_URL"); v != "" {
		c.OpenAIBaseURL = v
	}

	// Tool output
	if v := os.Getenv("PI_GO_MAX_OUTPUT_LEN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.MaxOutputLen = n
		}
	}

	// Execution backend
	if v := os.Getenv("PI_GO_EXECUTION_MODE"); v != "" {
		c.ExecutionMode = v
	}
	if v := os.Getenv("PI_GO_SSH_HOST"); v != "" {
		c.SSHHost = v
	}
	if v := os.Getenv("PI_GO_SSH_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			c.SSHPort = p
		}
	}
	if v := os.Getenv("PI_GO_SSH_WORKDIR"); v != "" {
		c.SSHWorkDir = v
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

	// Music
	if v := os.Getenv("PI_GO_MUSIC_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			c.MusicPort = p
		}
	}
}

// LoadDotEnv 从 .env 文件读取键值对，设置到环境变量中。
// 已有的环境变量不会被覆盖（即环境变量优先级 > .env 文件）。
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
		// 不覆盖已有环境变量
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
	return nil
}
