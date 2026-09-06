package sdk_test

import (
	"context"
	"fmt"

	"github.com/hwj123hwj/pi-go/sdk/agent"
	"github.com/hwj123hwj/pi-go/sdk/ai"
	"github.com/hwj123hwj/pi-go/sdk/ai/providers"
)

// echoProvider 演示实现 providers.Provider 的最小样子：不调网络，直接回固定文本。
// 真实场景换成 anthropic / openai 等注册进同一个 Registry。
type echoProvider struct{}

func (echoProvider) Name() string { return "echo" }

func (echoProvider) StreamSimple(ctx context.Context, req ai.SimpleStreamRequest) (*ai.EventStream, error) {
	return echoProvider{}.Stream(ctx, ai.StreamRequest{
		Messages: req.Messages,
		System:   req.System,
	})
}

func (echoProvider) Stream(ctx context.Context, req ai.StreamRequest) (*ai.EventStream, error) {
	stream := ai.NewEventStream(8)
	go func() {
		defer stream.Close()
		partial := ai.StreamAssistantMessage{
			Text:       "你好，我是被嵌入的 agent",
			StopReason: ai.StopReasonStop,
		}
		_ = stream.Push(ctx, ai.EventStart{Partial: partial})
		_ = stream.Push(ctx, ai.EventTextDelta{ContentIndex: 0, Delta: partial.Text, Partial: partial})
		_ = stream.Push(ctx, ai.EventDone{Reason: ai.StopReasonStop, Message: partial})
		stream.SetResult(partial, nil)
	}()
	return stream, nil
}

// Example_agent 演示 SDK 的最小用法：注册 Provider → 组装 Agent → 一轮对话。
// 这就是"在自己的 Go 后端服务里拿到 pi-go 原子能力"的全部代码。
func Example_agent() {
	registry := providers.NewRegistry()
	registry.Register(echoProvider{})

	ag := agent.New(agent.Options{
		Model:    ai.Model{ID: "echo-1", Name: "echo", Provider: "echo"},
		Registry: registry,
		System:   "你是一个嵌入式助手",
	})

	reply, err := ag.Prompt(context.Background(), ai.NewTextUserMessage("打个招呼"))
	if err != nil {
		fmt.Println("err:", err)
		return
	}
	fmt.Println(reply.Text)
	// Output: 你好，我是被嵌入的 agent
}
