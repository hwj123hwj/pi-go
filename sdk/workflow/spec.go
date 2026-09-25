// Package workflow 提供声明式多步骤 Agent 工作流编排：
// YAML 定义步骤 DAG（顺序依赖、fan-out、重试、人工确认门），
// 引擎按依赖调度执行，每步通过 RunnerFactory 驱动一次 Agent 会话；
// 执行全程写 JSONL journal，步骤按内容签名缓存（重跑不重复付费）。
package workflow

import (
	"fmt"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Status 值用于 RunMeta.Status 与 StepMeta.Status。
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
	StatusRejected  = "rejected" // 确认门被拒绝
	StatusWaiting   = "waiting_approval"
	StatusSkipped   = "skipped" // 上游步骤失败而未执行
)

// Spec 是工作流的声明式定义（YAML）。
type Spec struct {
	Name           string         `yaml:"name" json:"name"`
	Description    string         `yaml:"description,omitempty" json:"description,omitempty"`
	Timeout        string         `yaml:"timeout,omitempty" json:"timeout,omitempty"` // e.g. "30m"
	MaxConcurrency int            `yaml:"max_concurrency,omitempty" json:"max_concurrency,omitempty"`
	Vars           map[string]any `yaml:"vars,omitempty" json:"vars,omitempty"`
	Steps          []Step         `yaml:"steps" json:"steps"`
}

// Step 是单个编排步骤。
type Step struct {
	ID          string `yaml:"id" json:"id"`
	Prompt      string `yaml:"prompt" json:"prompt"` // 模板：{{vars.x}} {{steps.id.output}} {{item}} {{index}}
	Model       string `yaml:"model,omitempty" json:"model,omitempty"`
	Retries     int    `yaml:"retries,omitempty" json:"retries,omitempty"`
	Confirm     bool   `yaml:"confirm,omitempty" json:"confirm,omitempty"`   // 执行前等待人工审批
	DependsOn   []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty"` // 省略时默认依赖上一个步骤
	Foreach     string `yaml:"foreach,omitempty" json:"foreach,omitempty"` // 变量名或模板 → 列表，fan-out
	Concurrency int    `yaml:"concurrency,omitempty" json:"concurrency,omitempty"` // foreach 并发上限
}

// ParseSpec 解析 YAML 并校验。
func ParseSpec(data []byte) (*Spec, error) {
	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse workflow yaml: %w", err)
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return &spec, nil
}

var stepIDRe = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// Validate 校验 spec：必填字段、ID 唯一、依赖存在且无环、foreach 不允许 confirm。
func (s *Spec) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if len(s.Steps) == 0 {
		return fmt.Errorf("workflow requires at least one step")
	}
	if s.MaxConcurrency < 0 || s.MaxConcurrency > 64 {
		return fmt.Errorf("max_concurrency must be in 1..64")
	}
	if s.Timeout != "" {
		if _, err := time.ParseDuration(s.Timeout); err != nil {
			return fmt.Errorf("invalid timeout %q: %w", s.Timeout, err)
		}
	}

	ids := make(map[string]bool, len(s.Steps))
	for i := range s.Steps {
		st := &s.Steps[i]
		if !stepIDRe.MatchString(st.ID) {
			return fmt.Errorf("step %d: id %q must match %s", i, st.ID, stepIDRe)
		}
		if ids[st.ID] {
			return fmt.Errorf("duplicate step id %q", st.ID)
		}
		ids[st.ID] = true
		if st.Prompt == "" {
			return fmt.Errorf("step %q: prompt is required", st.ID)
		}
		if st.Retries < 0 || st.Retries > 10 {
			return fmt.Errorf("step %q: retries must be in 0..10", st.ID)
		}
		if st.Concurrency < 0 || st.Concurrency > 64 {
			return fmt.Errorf("step %q: concurrency must be in 1..64", st.ID)
		}
		if st.Confirm && st.Foreach != "" {
			return fmt.Errorf("step %q: confirm is not supported on foreach steps", st.ID)
		}
	}

	// 依赖存在性 + 环检测；省略 depends_on 的步骤隐式依赖上一个步骤（顺序链）。
	deps := make(map[string][]string, len(s.Steps))
	var prev string
	for i := range s.Steps {
		st := &s.Steps[i]
		if len(st.DependsOn) == 0 && prev != "" {
			deps[st.ID] = []string{prev}
		} else {
			deps[st.ID] = st.DependsOn
		}
		for _, d := range deps[st.ID] {
			if !ids[d] {
				return fmt.Errorf("step %q: unknown dependency %q", st.ID, d)
			}
		}
		prev = st.ID
	}
	if cycle := findCycle(deps); cycle != "" {
		return fmt.Errorf("dependency cycle involving %q", cycle)
	}
	return nil
}

// findCycle 返回环上任意一个节点，无环返回空串（三色标记）。
func findCycle(deps map[string][]string) string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(deps))
	var dfs func(string) string
	dfs = func(id string) string {
		color[id] = gray
		for _, d := range deps[id] {
			switch color[d] {
			case gray:
				return d
			case white:
				if c := dfs(d); c != "" {
					return c
				}
			}
		}
		color[id] = black
		return ""
	}
	for id := range deps {
		if color[id] == white {
			if c := dfs(id); c != "" {
				return c
			}
		}
	}
	return ""
}

// stepDeps 返回每个步骤的最终依赖表（含隐式顺序链），Validate 已保证合法性。
func (s *Spec) stepDeps() map[string][]string {
	deps := make(map[string][]string, len(s.Steps))
	var prev string
	for i := range s.Steps {
		st := &s.Steps[i]
		if len(st.DependsOn) == 0 && prev != "" {
			deps[st.ID] = []string{prev}
		} else {
			deps[st.ID] = append([]string(nil), st.DependsOn...)
		}
		prev = st.ID
	}
	return deps
}
