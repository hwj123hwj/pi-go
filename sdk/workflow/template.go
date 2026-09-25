package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// StepOutput 是模板可见的步骤产出。
type StepOutput struct {
	Output  string   `json:"output"`            // 全部产出（foreach 为各项 join）
	Outputs []string `json:"outputs,omitempty"` // foreach 各项原始输出
	Cached  bool     `json:"cached,omitempty"`
}

// 模板语法归一化：用户写 {{vars.x}} / {{steps.a.output}} / {{item}} / {{index}}，
// text/template 的 map 访问要求点前缀，这里在解析前统一补上。
// {{index ...}} 是模板内建函数，仅改写不含参数的 {{index}}，二者不冲突。
var (
	dottedRootRe = regexp.MustCompile(`\{\{\s*(vars|steps)\.`)
	bareSymbolRe = regexp.MustCompile(`\{\{\s*(item|index)\s*\}\}`)
)

func normalizeTemplate(prompt string) string {
	prompt = dottedRootRe.ReplaceAllString(prompt, `{{.$1.`)
	return bareSymbolRe.ReplaceAllString(prompt, `{{.$1}}`)
}

// renderPrompt 渲染步骤模板。数据面：
//   - vars   用户变量（spec.vars 运行时覆盖后的值）
//   - steps  已完成步骤产出（{{steps.a.output}} / {{steps.a.outputs}} / {{steps.a.cached}}；
//     引用未完成步骤会报错）
//   - item / index  foreach 当前项与序号（非 foreach 步骤中引用会报错）
//
// missingkey=error 保证引用不存在的键立刻报错而不是渲染出 "<no value>"。
func renderPrompt(prompt string, vars map[string]any, outputs map[string]StepOutput, item any, index int) (string, error) {
	stepsData := make(map[string]any, len(outputs))
	for id, out := range outputs {
		stepsData[id] = map[string]any{
			"output":  out.Output,
			"outputs": out.Outputs,
			"cached":  out.Cached,
		}
	}
	data := map[string]any{
		"vars":  vars,
		"steps": stepsData,
	}
	if item != nil {
		data["item"] = item
		data["index"] = index
	}
	tmpl, err := template.New("prompt").Option("missingkey=error").Parse(normalizeTemplate(prompt))
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}
	return buf.String(), nil
}

// resolveForeach 解析 foreach 字段为项目列表。两种写法：
//  1. 纯变量名（不含 "{{"）：取 vars[name]，要求是数组，保留元素类型
//  2. 模板字符串：渲染后先尝试 JSON 数组，否则按行切分（去空行）
func resolveForeach(foreach string, vars map[string]any, outputs map[string]StepOutput) ([]any, error) {
	if !strings.Contains(foreach, "{{") {
		v, ok := vars[foreach]
		if !ok {
			return nil, fmt.Errorf("foreach: unknown var %q", foreach)
		}
		switch list := v.(type) {
		case []any:
			return list, nil
		case []string:
			out := make([]any, len(list))
			for i, s := range list {
				out[i] = s
			}
			return out, nil
		default:
			return nil, fmt.Errorf("foreach: var %q is %T, want list", foreach, v)
		}
	}

	rendered, err := renderPrompt(foreach, vars, outputs, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("foreach: %w", err)
	}
	trimmed := strings.TrimSpace(rendered)
	if strings.HasPrefix(trimmed, "[") {
		var arr []any
		if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
			return arr, nil
		}
	}
	var lines []any
	for _, line := range strings.Split(rendered, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

// stepOutputSnapshot 返回已完成步骤产出的深拷贝。
func stepOutputSnapshot(outputs map[string]StepOutput) map[string]StepOutput {
	snap := make(map[string]StepOutput, len(outputs))
	for k, v := range outputs {
		v.Outputs = append([]string(nil), v.Outputs...)
		snap[k] = v
	}
	return snap
}
