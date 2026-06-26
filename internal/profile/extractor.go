package profile

// ────────────────────────────────────────────────────────────────────────────
//  Session Memory Extractor
//
//  Adapted from OpenViking's SessionCompressorV2.extract_long_term_memories():
//  After a conversation ends, scan the transcript for extractable user facts,
//  and use the LLM to condense them into structured profile entries.
//
//  Key design differences from OpenViking:
//  1. No VLM/ExtractLoop templating — we use a single LLM call with a
//     structured JSON output prompt (much simpler, sufficient for single-user)
//  2. No trajectory/experience memory — we only extract USER facts, not
//     agent execution patterns (desktop agent doesn't need those)
//  3. Non-blocking — extraction runs in a goroutine so it never delays
//     the user's response
//
//  Cost management:
//  - Only the last N messages are scanned (not full history)
//  - Messages are truncated to keep the prompt under ~2K tokens
//  - Extraction runs at most once per session close, not per turn
// ────────────────────────────────────────────────────────────────────────────

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ExtractableFact is a single fact extracted from conversation.
type ExtractableFact struct {
	Category string `json:"category"` // "coding", "music", "general"
	Key      string `json:"key"`      // e.g. "language", "location"
	Value    string `json:"value"`    // e.g. "Go", "北京"
}

// LLMCaller is a minimal interface for making a non-streaming LLM call.
// This decouples the extractor from the full Provider interface.
type LLMCaller interface {
	// CallSimple sends a system + user prompt and returns the text response.
	CallSimple(ctx context.Context, system, userPrompt string) (string, error)
}

// SessionExtractor scans conversations and extracts user facts.
type SessionExtractor struct {
	profile *Store
	llm     LLMCaller
}

// NewSessionExtractor creates an extractor. llm can be nil (extraction disabled).
func NewSessionExtractor(profile *Store, llm LLMCaller) *SessionExtractor {
	return &SessionExtractor{profile: profile, llm: llm}
}

// ExtractFromMessages scans recent messages and extracts user facts.
// It takes the last N messages (to limit token usage) and asks the LLM
// to identify facts about the user.
//
// This is the Go adaptation of OpenViking's extract_long_term_memories.
// We deliberately keep it simple: one LLM call, structured JSON output.
func (e *SessionExtractor) ExtractFromMessages(ctx context.Context, recentMessages []MessageSnippet) {
	if e.llm == nil || e.profile == nil {
		return
	}
	if len(recentMessages) == 0 {
		return
	}

	// Build conversation transcript (truncated to keep prompt small)
	var transcript strings.Builder
	const maxTranscriptChars = 4000
	for _, msg := range recentMessages {
		if msg.Role == "tool" {
			continue // skip tool results (too verbose, low signal)
		}
		line := fmt.Sprintf("[%s] %s\n", msg.Role, truncate(msg.Content, 500))
		if transcript.Len()+len(line) > maxTranscriptChars {
			transcript.WriteString("...(truncated)...\n")
			break
		}
		transcript.WriteString(line)
	}

	if transcript.Len() == 0 {
		return
	}

	system := `你是一个信息提取助手。分析用户与AI助手的对话记录，提取关于用户的持久性事实。
只提取明确的、确定性的用户信息，不要猜测或推断。

支持的类别：
- coding: 编程语言、工具、操作系统、编辑器等（如 language: Go, editor: vim, os: macOS）
- music: 音乐偏好（如 genre: 摇滚, artist: 周杰伦）
- general: 位置、时区、语言偏好等（如 location: 北京, language: 中文）

规则：
1. 只提取用户明确陈述的事实（"我用Go"、"我在北京"），不猜测
2. 忽略临时性的对话内容（如"帮我写个排序算法"不是用户画像）
3. 忽略AI助手的回复，只看用户说了什么
4. 如果没有可提取的事实，返回空数组`

	userPrompt := fmt.Sprintf(`请从以下对话中提取用户事实，返回JSON格式：
{"facts": [{"category": "coding", "key": "language", "value": "Go"}, ...]}

如果没有可提取的事实，返回：{"facts": []}

对话记录：
%s`, transcript.String())

	// Call LLM with timeout
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	response, err := e.llm.CallSimple(callCtx, system, userPrompt)
	if err != nil {
		slog.Debug("session memory extraction LLM call failed", "error", err)
		return
	}

	// Parse JSON response
	var result struct {
		Facts []ExtractableFact `json:"facts"`
	}
	// Try to extract JSON from response (LLM may wrap it in markdown)
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		slog.Debug("session memory extraction JSON parse failed", "error", err)
		return
	}

	// Record extracted facts
	for _, fact := range result.Facts {
		if fact.Category == "" || fact.Key == "" || fact.Value == "" {
			continue
		}
		// Validate category
		if fact.Category != CategoryCoding && fact.Category != CategoryMusic && fact.Category != CategoryGeneral {
			continue
		}
		e.profile.Record(fact.Category, fact.Key, fact.Value, "session-extract")
	}

	if len(result.Facts) > 0 {
		slog.Info("session memory extracted", "count", len(result.Facts))
	}
}

// MessageSnippet is a minimal message representation for extraction.
type MessageSnippet struct {
	Role    string // "user", "assistant", "tool"
	Content string
}

// ExtractAsync runs extraction in a goroutine (non-blocking).
// Used after a conversation turn completes.
func (e *SessionExtractor) ExtractAsync(recentMessages []MessageSnippet) {
	if e.llm == nil || e.profile == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		e.ExtractFromMessages(ctx, recentMessages)
	}()
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractJSON finds the first JSON object in a string that may be wrapped
// in markdown code blocks.
func extractJSON(s string) string {
	// Try to find ```json ... ``` block
	if idx := strings.Index(s, "```json"); idx != -1 {
		start := idx + 7
		if end := strings.Index(s[start:], "```"); end != -1 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	// Try to find ``` ... ``` block
	if idx := strings.Index(s, "```"); idx != -1 {
		start := idx + 3
		if end := strings.Index(s[start:], "```"); end != -1 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	// Try to find raw { ... } object
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}
	return ""
}
