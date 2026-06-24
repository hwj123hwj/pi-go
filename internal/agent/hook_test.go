package agent

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/hwj123hwj/pi-go/internal/ai"
	"github.com/hwj123hwj/pi-go/internal/ai/providers"
	"github.com/hwj123hwj/pi-go/internal/compaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SessionStart/SessionEnd hook 在一次 Prompt 中各触发一次，且 error 不阻断主流程。
func TestSessionHooks_TriggeredOncePerPrompt(t *testing.T) {
	var mu sync.Mutex
	var startCount, endCount int
	var endErr error

	mp := &mockTestProvider{responses: []mockTestResponse{
		{text: "hi", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(mp)

	ag := New(Options{
		Model:    ai.Model{ID: "t", Name: "t", Provider: "mock_test"},
		Registry: registry,
		System:   "test",
		MaxTurns: 3,
		LifecycleHooks: LifecycleHooks{
			SessionStart: []SessionStartHook{func(ctx context.Context, e SessionStartEvent) error {
				mu.Lock()
				startCount++
				mu.Unlock()
				return nil
			}},
			SessionEnd: []SessionEndHook{func(ctx context.Context, e SessionEndEvent) error {
				mu.Lock()
				endCount++
				endErr = e.Err
				mu.Unlock()
				return nil
			}},
		},
	})

	_, err := ag.Prompt(context.Background(), ai.NewTextUserMessage("hello"))
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, startCount, "SessionStart 应触发一次")
	assert.Equal(t, 1, endCount, "SessionEnd 应触发一次")
	assert.Nil(t, endErr, "正常结束时 Err 应为 nil")
}

// SessionStart hook 返回 error 时不阻断主流程（会话照常完成）。
func TestSessionHooks_ErrorDoesNotBlock(t *testing.T) {
	var endCalled bool
	mp := &mockTestProvider{responses: []mockTestResponse{
		{text: "ok", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(mp)

	ag := New(Options{
		Model:    ai.Model{ID: "t", Name: "t", Provider: "mock_test"},
		Registry: registry,
		System:   "test",
		MaxTurns: 3,
		LifecycleHooks: LifecycleHooks{
			SessionStart: []SessionStartHook{func(ctx context.Context, e SessionStartEvent) error {
				return errString("simulated hook failure")
			}},
			SessionEnd: []SessionEndHook{func(ctx context.Context, e SessionEndEvent) error {
				endCalled = true
				return nil
			}},
		},
	})

	assistant, err := ag.Prompt(context.Background(), ai.NewTextUserMessage("hi"))
	require.NoError(t, err, "hook error 不应阻断会话")
	assert.Equal(t, "ok", assistant.Text)
	assert.True(t, endCalled, "SessionEnd 仍应触发")
}

// PreCompress hook 在上下文压缩前触发，收到正确的 token/window/messageCount。
func TestPreCompressHook_TriggeredBeforeCompaction(t *testing.T) {
	var mu sync.Mutex
	var got PreCompressEvent
	var triggered bool

	// mock provider：返回纯文本，但历史超长会触发压缩
	mp := &mockTestProvider{responses: []mockTestResponse{
		{text: "done", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(mp)

	// 极小的 contextWindow（1000）+ 极小的 KeepRecentTokens，让短历史也触发压缩
	settings := compaction.Settings{
		Enabled:          true,
		ReserveTokens:    100,
		KeepRecentTokens: 50,
	}

	ag := New(Options{
		Model: ai.Model{
			ID:            "t",
			Name:          "t",
			Provider:      "mock_test",
			ContextWindow: 1000, // 小窗口，易触发
		},
		Registry:           registry,
		System:             "test",
		MaxTurns:           3,
		CompactionSettings: settings,
		// mock 摘要函数，避免真调 LLM
		SummarizeFunc: func(ctx context.Context, history []ai.Message, recent []ai.Message, customInstructions string) (string, error) {
			return "mocked summary", nil
		},
		LifecycleHooks: LifecycleHooks{
			PreCompress: []PreCompressHook{func(ctx context.Context, e PreCompressEvent) error {
				mu.Lock()
				triggered = true
				got = e
				mu.Unlock()
				return nil
			}},
		},
	})

	// 预填超长历史（user/assistant 交替，SplitMessages 要求 turn 边界才能切割），
	// 让 maybeCompact 触发压缩。500 turns × 2 ≈ 10000 token，远超阈值 900。
	longHistory := make([]ai.Message, 0, 1000)
	for i := 0; i < 500; i++ {
		longHistory = append(longHistory, ai.NewTextUserMessage(strings.Repeat("x", 40)))
		longHistory = append(longHistory, ai.AssistantMessage{Text: strings.Repeat("y", 40), StopReason: ai.StopReasonStop})
	}

	result := ag.maybeCompact(context.Background(), longHistory)

	mu.Lock()
	defer mu.Unlock()
	assert.True(t, triggered, "PreCompress 应被触发")
	assert.NotEqual(t, len(longHistory), len(result), "history 应被压缩")
	assert.Greater(t, got.ContextTokens, 0)
	assert.Equal(t, 1000, got.ContextWindow)
	assert.Greater(t, got.MessageCount, 0)
}

// errString 是测试用的简单 error 类型。
type errString string

func (e errString) Error() string { return string(e) }
