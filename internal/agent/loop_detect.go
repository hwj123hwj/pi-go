package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// LoopDetectSettings 控制循环检测行为（参考 compaction.Settings 的 Settings 惯例）。
//
// 检测目标：Agent 连续重复调用"相同工具 + 相同参数"（典型空转：反复 read 同一文件、
// 反复 grep 同一 pattern）。检测到后柔性干预——向 followUpQueue 注入提醒让 Agent 自救，
// 不强制中断。
type LoopDetectSettings struct {
	Enabled          bool   // 是否启用循环检测
	Threshold        int    // 连续重复多少次触发（默认 5）
	ReminderTemplate string // 注入给 LLM 的提醒模板，第一个 %s=工具名，第二个 %s=参数摘要，%d=重复次数
}

// DefaultLoopDetectSettings 返回保守的默认配置。
func DefaultLoopDetectSettings() LoopDetectSettings {
	return LoopDetectSettings{
		Enabled:   true,
		Threshold: 5,
		ReminderTemplate: "You have called tool %q with the same arguments %d times in a row — " +
			"this looks like a loop. Step back, reconsider your approach, and try a different method. " +
			"(repeated args summary: %s)",
	}
}

// loopDetector 跟踪连续相同的 tool call，per-prompt 生命周期。
// 指纹用 SHA256(name + ":" + args)，与 DeepV loopDetectionService.getToolCallKey 一致。
type loopDetector struct {
	lastFingerprint string // 上一个 tool call 的指纹；空串表示尚无记录
	repeatCount     int    // 当前指纹的连续重复次数
}

// observe 记录一次 tool call，返回是否检测到循环（连续重复 >= Threshold）。
// name/args 来自 ai.ToolCall。指纹含 args，故同工具不同参数不算重复。
func (d *loopDetector) observe(threshold int, name, args string) bool {
	fp := fingerprint(name, args)
	if fp == d.lastFingerprint {
		d.repeatCount++
	} else {
		d.lastFingerprint = fp
		d.repeatCount = 1
	}
	return d.repeatCount >= threshold
}

// reset 清空状态，用于每次新 prompt 开始时（per-prompt 计数，不跨对话累积）。
func (d *loopDetector) reset() {
	d.lastFingerprint = ""
	d.repeatCount = 0
}

// fingerprint 计算 tool call 的指纹：SHA256(name + ":" + args)。
func fingerprint(name, args string) string {
	h := sha256.Sum256([]byte(name + ":" + args))
	return hex.EncodeToString(h[:])
}

// summarizeArgs 生成参数的人类可读摘要，用于提醒文案。截断避免过长。
func summarizeArgs(args string) string {
	s := strings.TrimSpace(args)
	if len(s) > 80 {
		return s[:80] + "..."
	}
	if s == "" {
		return "(none)"
	}
	return s
}

// renderReminder 用 ReminderTemplate 渲染提醒文案。
func renderReminder(tmpl, toolName, args string, count int) string {
	return fmt.Sprintf(tmpl, toolName, count, summarizeArgs(args))
}
