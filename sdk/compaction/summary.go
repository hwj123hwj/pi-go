package compaction

import (
	"context"
	"fmt"

	"github.com/hwj123hwj/pi-go/sdk/ai"
	"github.com/hwj123hwj/pi-go/sdk/ai/providers"
)

// LLMSummarizer 返回一个使用 LLM 生成摘要的 SummarizeFunc。
// 它通过 Provider 发起一次简单的 LLM 调用来生成摘要。
func LLMSummarizer(registry *providers.Registry, model ai.Model) SummarizeFunc {
	return func(ctx context.Context, history []ai.Message, recent []ai.Message, customInstructions string) (string, error) {
		provider, ok := registry.Get(model.Provider)
		if !ok {
			return "", fmt.Errorf("provider %q not found for summarization", model.Provider)
		}

		prompt := SummarizePrompt(history, customInstructions)

		req := ai.StreamRequest{
			Model: model,
			Messages: []ai.Message{
				ai.NewTextUserMessage(prompt),
			},
			System:   "You are a conversation summarizer. Be concise but thorough.",
			MaxTokens: nil,
		}

		stream, err := provider.Stream(ctx, req)
		if err != nil {
			return "", fmt.Errorf("summarization LLM call failed: %w", err)
		}

		var result ai.StreamAssistantMessage
		for event := range stream.Events() {
			switch e := event.(type) {
			case ai.EventDone:
				result = e.Message
			case ai.EventError:
				return "", fmt.Errorf("summarization stream error: %s", e.Error)
			}
		}

		return result.Text, nil
	}
}
