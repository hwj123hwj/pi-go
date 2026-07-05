package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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
	EnableWebSearch    bool

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

	// Knowledge base (second-brain)
	KBRepoPath string // path to the personal knowledge repo (default: ~/agent-lessons)

	// KB vector search (optional, for semantic search)
	KBEmbeddingAPIKey string // API key for embedding provider (e.g. SiliconFlow)
	KBEmbeddingModel  string // embedding model name (e.g. BAAI/bge-m3)
	KBEmbeddingBaseURL string // embedding API base URL

	// ASR (speech-to-text)
	ASRAPIKey  string // API key for SiliconFlow ASR
	ASRModel   string // ASR model name (default: TeleAI/TeleSpeechASR)
	ASRBaseURL string // ASR API base URL (default: https://api.siliconflow.cn)

	// Server security
	APIKey string // Bearer token for HTTP API auth (empty = no auth, backward compatible)
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
		EnableWebSearch:    false,

		MaxOutputLen: 30000,

		MusicPort: 8081,

		KBRepoPath: "", // empty → defaults to ~/agent-lessons at runtime
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
	if v := os.Getenv("PI_GO_ENABLE_WEB_SEARCH"); v != "" {
		c.EnableWebSearch = strings.ToLower(v) == "true" || v == "1"
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

	// Knowledge base
	if v := os.Getenv("PI_GO_KB_REPO_PATH"); v != "" {
		c.KBRepoPath = v
	}

	// KB vector search
	if v := os.Getenv("SILICONFLOW_API_KEY"); v != "" {
		c.KBEmbeddingAPIKey = v
	}
	if v := os.Getenv("SILICONFLOW_EMBEDDING_MODEL"); v != "" {
		c.KBEmbeddingModel = v
	}
	if v := os.Getenv("SILICONFLOW_BASE_URL"); v != "" {
		c.KBEmbeddingBaseURL = v
	}

	// ASR (speech-to-text) — reuse SiliconFlow API key by default
	if v := os.Getenv("ASR_API_KEY"); v != "" {
		c.ASRAPIKey = v
	} else if v := os.Getenv("SILICONFLOW_API_KEY"); v != "" {
		c.ASRAPIKey = v // reuse SiliconFlow key
	}
	if v := os.Getenv("ASR_MODEL"); v != "" {
		c.ASRModel = v
	} else {
		c.ASRModel = "TeleAI/TeleSpeechASR" // default
	}
	if v := os.Getenv("ASR_BASE_URL"); v != "" {
		c.ASRBaseURL = v
	} else {
		c.ASRBaseURL = "https://api.siliconflow.cn"
	}

	// Server security
	if v := os.Getenv("PI_GO_API_KEY"); v != "" {
		c.APIKey = v
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

// yamlConfig is the YAML representation of Config for file-based configuration.
// Uses yaml tags so users can write pi-go.yaml instead of 30+ env vars.
type yamlConfig struct {
	Name        string `yaml:"name,omitempty"`
	Host        string `yaml:"host,omitempty"`
	Port        int    `yaml:"port,omitempty"`
	DataDir     string `yaml:"data_dir,omitempty"`
	SessionFile string `yaml:"session_file,omitempty"`

	Provider string `yaml:"provider,omitempty"`

	AnthropicAPIKey  string `yaml:"anthropic_api_key,omitempty"`
	AnthropicModel   string `yaml:"anthropic_model,omitempty"`
	AnthropicBaseURL string `yaml:"anthropic_base_url,omitempty"`

	OpenAIAPIKey  string `yaml:"openai_api_key,omitempty"`
	OpenAIModel   string `yaml:"openai_model,omitempty"`
	OpenAIBaseURL string `yaml:"openai_base_url,omitempty"`

	Workspace          string `yaml:"workspace,omitempty"`
	EnableBash         bool   `yaml:"enable_bash,omitempty"`
	BashTimeoutSeconds int    `yaml:"bash_timeout_seconds,omitempty"`
	EnableWeb          bool   `yaml:"enable_web,omitempty"`
	WebTimeoutSeconds  int    `yaml:"web_timeout_seconds,omitempty"`
	EnableWebSearch    bool   `yaml:"enable_web_search,omitempty"`

	ExecutionMode string `yaml:"execution_mode,omitempty"`
	SSHHost       string `yaml:"ssh_host,omitempty"`
	SSHPort       int    `yaml:"ssh_port,omitempty"`
	SSHWorkDir    string `yaml:"ssh_work_dir,omitempty"`

	MaxOutputLen int      `yaml:"max_output_len,omitempty"`
	AllowedTools []string `yaml:"allowed_tools,omitempty"`
	BlockedTools []string `yaml:"blocked_tools,omitempty"`

	MaxTurns int           `yaml:"max_turns,omitempty"`
	Timeout  time.Duration `yaml:"timeout,omitempty"`

	MusicPort int    `yaml:"music_port,omitempty"`
	APIKey    string `yaml:"api_key,omitempty"`

	KBRepoPath         string `yaml:"kb_repo_path,omitempty"`
	KBEmbeddingAPIKey  string `yaml:"kb_embedding_api_key,omitempty"`
	KBEmbeddingModel   string `yaml:"kb_embedding_model,omitempty"`
	KBEmbeddingBaseURL string `yaml:"kb_embedding_base_url,omitempty"`

	ASRAPIKey  string `yaml:"asr_api_key,omitempty"`
	ASRModel   string `yaml:"asr_model,omitempty"`
	ASRBaseURL string `yaml:"asr_base_url,omitempty"`
}

// LoadFromYAML loads configuration from a YAML file.
// This is the lowest-priority source: values here are overridden by env vars.
// Call this BEFORE LoadFromEnv for correct precedence.
func (c *Config) LoadFromYAML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	var yc yamlConfig
	if err := yaml.Unmarshal(data, &yc); err != nil {
		return fmt.Errorf("parse YAML: %w", err)
	}

	// Apply YAML values only if non-zero (env vars override later)
	if yc.Name != "" {
		c.Name = yc.Name
	}
	if yc.Host != "" {
		c.Host = yc.Host
	}
	if yc.Port != 0 {
		c.Port = yc.Port
	}
	if yc.DataDir != "" {
		c.DataDir = yc.DataDir
	}
	if yc.SessionFile != "" {
		c.SessionFile = yc.SessionFile
	}
	if yc.Provider != "" {
		c.Provider = yc.Provider
	}
	if yc.AnthropicAPIKey != "" {
		c.AnthropicAPIKey = yc.AnthropicAPIKey
	}
	if yc.AnthropicModel != "" {
		c.AnthropicModel = yc.AnthropicModel
	}
	if yc.AnthropicBaseURL != "" {
		c.AnthropicBaseURL = yc.AnthropicBaseURL
	}
	if yc.OpenAIAPIKey != "" {
		c.OpenAIAPIKey = yc.OpenAIAPIKey
	}
	if yc.OpenAIModel != "" {
		c.OpenAIModel = yc.OpenAIModel
	}
	if yc.OpenAIBaseURL != "" {
		c.OpenAIBaseURL = yc.OpenAIBaseURL
	}
	if yc.Workspace != "" {
		c.Workspace = yc.Workspace
	}
	if yc.EnableBash {
		c.EnableBash = true
	}
	if yc.BashTimeoutSeconds > 0 {
		c.BashTimeoutSeconds = yc.BashTimeoutSeconds
	}
	if yc.EnableWeb {
		c.EnableWeb = true
	}
	if yc.WebTimeoutSeconds > 0 {
		c.WebTimeoutSeconds = yc.WebTimeoutSeconds
	}
	if yc.EnableWebSearch {
		c.EnableWebSearch = true
	}
	if yc.ExecutionMode != "" {
		c.ExecutionMode = yc.ExecutionMode
	}
	if yc.SSHHost != "" {
		c.SSHHost = yc.SSHHost
	}
	if yc.SSHPort > 0 {
		c.SSHPort = yc.SSHPort
	}
	if yc.SSHWorkDir != "" {
		c.SSHWorkDir = yc.SSHWorkDir
	}
	if yc.MaxOutputLen > 0 {
		c.MaxOutputLen = yc.MaxOutputLen
	}
	if len(yc.AllowedTools) > 0 {
		c.AllowedTools = yc.AllowedTools
	}
	if len(yc.BlockedTools) > 0 {
		c.BlockedTools = yc.BlockedTools
	}
	if yc.MaxTurns > 0 {
		c.MaxTurns = yc.MaxTurns
	}
	if yc.Timeout > 0 {
		c.Timeout = yc.Timeout
	}
	if yc.MusicPort > 0 {
		c.MusicPort = yc.MusicPort
	}
	if yc.APIKey != "" {
		c.APIKey = yc.APIKey
	}
	if yc.KBRepoPath != "" {
		c.KBRepoPath = yc.KBRepoPath
	}
	if yc.KBEmbeddingAPIKey != "" {
		c.KBEmbeddingAPIKey = yc.KBEmbeddingAPIKey
	}
	if yc.KBEmbeddingModel != "" {
		c.KBEmbeddingModel = yc.KBEmbeddingModel
	}
	if yc.KBEmbeddingBaseURL != "" {
		c.KBEmbeddingBaseURL = yc.KBEmbeddingBaseURL
	}
	if yc.ASRAPIKey != "" {
		c.ASRAPIKey = yc.ASRAPIKey
	}
	if yc.ASRModel != "" {
		c.ASRModel = yc.ASRModel
	}
	if yc.ASRBaseURL != "" {
		c.ASRBaseURL = yc.ASRBaseURL
	}

	return nil
}
