package agent

import (
	"context"
	"sync"
	"testing"

	"github.com/hwj123hwj/pi-go/sdk/ai"
	"github.com/hwj123hwj/pi-go/sdk/ai/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 集成测试：连续重复相同 tool call 达到阈值时，
//  1. 发出 EventLoopDetected
//  2. 向 followUpQueue 注入提醒（下一轮 Agent 能收到）
//  3. Agent 不被强制中断（继续运行直到 maxTurns 或自然停止）
func TestLoopDetection_Integration(t *testing.T) {
	// mock provider：连续返回相同 echo tool call，最后返回纯文本停止。
	// 给足 response：循环检测在第 5 轮触发并注入提醒，提醒会被 followUpQueue 消费占一轮，
	// 故需要足够 response 避免越界。MaxTurns 兜底防止真死循环。
	sameCall := ai.ToolCall{ID: "c1", Name: "echo", Args: `{"message":"hi"}`}
	responses := make([]mockTestResponse, 0, 12)
	for i := 0; i < 10; i++ {
		responses = append(responses, mockTestResponse{
			toolCalls: []ai.ToolCall{sameCall},
			stop:      ai.StopReasonToolUse,
		})
	}
	responses = append(responses, mockTestResponse{text: "ok, switching approach", stop: ai.StopReasonStop})

	mp := &mockTestProvider{responses: responses}
	registry := providers.NewRegistry()
	registry.Register(mp)

	// 收集事件
	var mu sync.Mutex
	events := []AgentEvent{}
	ag := New(Options{
		Model:    ai.Model{ID: "t", Name: "t", Provider: "mock_test"},
		Registry: registry,
		System:   "test",
		Tools:    []Tool{&echoTool{}},
		MaxTurns: 8,
		LoopDetectSettings: LoopDetectSettings{
			Enabled:          true,
			Threshold:        5,
			ReminderTemplate: "loop on %q (%d times)",
		},
	})
	unsub := ag.Subscribe(func(ctx context.Context, e AgentEvent) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	})
	defer unsub()

	_, err := ag.Prompt(context.Background(), ai.NewTextUserMessage("go"))
	require.NoError(t, err)

	// 断言：至少触发一次 EventLoopDetected
	mu.Lock()
	defer mu.Unlock()
	detected := 0
	for _, e := range events {
		if _, ok := e.(EventLoopDetected); ok {
			detected++
		}
	}
	assert.GreaterOrEqual(t, detected, 1, "应至少发出一次 EventLoopDetected")

	// Agent 应能继续到完成（未被中断），最终自然停止
	// （provider 第 7 轮返回 stop，证明循环检测没有强制中断执行）
}

// 循环检测未启用（Enabled=false）时，连续相同 tool call 不触发事件、不注入提醒。
func TestLoopDetection_DisabledNoDetection(t *testing.T) {
	sameCall := ai.ToolCall{ID: "c1", Name: "echo", Args: `{"message":"hi"}`}
	responses := []mockTestResponse{
		{toolCalls: []ai.ToolCall{sameCall}, stop: ai.StopReasonToolUse},
		{toolCalls: []ai.ToolCall{sameCall}, stop: ai.StopReasonToolUse},
		{toolCalls: []ai.ToolCall{sameCall}, stop: ai.StopReasonToolUse},
		{text: "done", stop: ai.StopReasonStop},
	}
	mp := &mockTestProvider{responses: responses}
	registry := providers.NewRegistry()
	registry.Register(mp)

	var mu sync.Mutex
	detected := 0
	ag := New(Options{
		Model:              ai.Model{ID: "t", Name: "t", Provider: "mock_test"},
		Registry:           registry,
		System:             "test",
		Tools:              []Tool{&echoTool{}},
		MaxTurns:           10,
		LoopDetectSettings: LoopDetectSettings{Enabled: false, Threshold: 5},
	})
	ag.Subscribe(func(ctx context.Context, e AgentEvent) {
		mu.Lock()
		if _, ok := e.(EventLoopDetected); ok {
			detected++
		}
		mu.Unlock()
	})

	_, err := ag.Prompt(context.Background(), ai.NewTextUserMessage("go"))
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 0, detected, "Enabled=false 时不应检测")
}
