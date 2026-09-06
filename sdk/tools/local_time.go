package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hwj123hwj/pi-go/sdk/agent"
)

// LocalTimeTool returns the current wall-clock local time.
// Supports an optional IANA timezone parameter.
type LocalTimeTool struct{}

type LocalTimeParams struct {
	Timezone string `json:"timezone,omitempty"`
}

func NewLocalTimeTool() *LocalTimeTool {
	return &LocalTimeTool{}
}

func (t *LocalTimeTool) Name() string { return "local_time" }

func (t *LocalTimeTool) Description() string {
	return "Returns the current wall-clock local time of the machine running the agent. " +
		"Use this tool to determine the current real-world time, record a start time at the beginning of a long task, " +
		"calculate elapsed task duration by comparing two readings, or timestamp checkpoints, logs, or todo updates. " +
		"The tool is fast, side-effect free, and never requires user confirmation. " +
		"Returned JSON fields: iso (ISO 8601 UTC timestamp), unix_ms (number), unix_s (number), timezone (IANA name), local (YYYY-MM-DD HH:MM:SS), weekday (English)."
}

func (t *LocalTimeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"timezone": map[string]any{
				"type":        "string",
				"description": "Optional IANA timezone name (e.g. \"Asia/Shanghai\", \"UTC\"). If omitted, uses the system local timezone.",
			},
		},
		"required": []string{},
	}
}

func (t *LocalTimeTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params LocalTimeParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if params.Timezone != "" {
		if _, err := time.LoadLocation(params.Timezone); err != nil {
			return nil, fmt.Errorf("invalid timezone %q: %w", params.Timezone, err)
		}
	}
	return json.Marshal(params)
}

// IsConcurrencySafe implements agent.ConcurrencySafeChecker.
func (t *LocalTimeTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *LocalTimeTool) Execute(_ context.Context, raw json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var params LocalTimeParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	now := time.Now()
	systemTz, _ := time.LoadLocation("Local")
	if systemTz == nil {
		systemTz = time.UTC
	}

	var loc *time.Location
	var warning string

	if params.Timezone != "" {
		var err error
		loc, err = time.LoadLocation(params.Timezone)
		if err != nil {
			loc = systemTz
			warning = fmt.Sprintf("Invalid timezone %q (%v); fell back to system timezone.", params.Timezone, err)
		}
	} else {
		loc = systemTz
	}

	localTime := now.In(loc)
	weekday := localTime.Weekday().String()
	localStr := localTime.Format("2006-01-02 15:04:05")
	tzName := loc.String()

	data := map[string]any{
		"iso":      now.UTC().Format(time.RFC3339),
		"unix_ms":  now.UnixMilli(),
		"unix_s":   now.Unix(),
		"timezone": tzName,
		"local":    localStr,
		"weekday":  weekday,
	}
	if warning != "" {
		data["warning"] = warning
	}

	jsonBytes, _ := json.Marshal(data)

	display := fmt.Sprintf("Local time: %s (%s, %s)", localStr, tzName, weekday)
	if warning != "" {
		display += " - " + warning
	}

	return agent.ToolResult{
		Content: string(jsonBytes),
	}, nil
}
