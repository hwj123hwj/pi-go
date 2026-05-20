package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventStream_PushAndReceive(t *testing.T) {
	stream := NewEventStream(8)

	go func() {
		defer stream.Close()
		_ = stream.Push(context.Background(), EventStart{Partial: StreamAssistantMessage{}})
		_ = stream.Push(context.Background(), EventTextDelta{Delta: "hello", Partial: StreamAssistantMessage{Text: "hello"}})
		_ = stream.Push(context.Background(), EventDone{Reason: StopReasonStop, Message: StreamAssistantMessage{Text: "hello"}})
		stream.SetResult(StreamAssistantMessage{Text: "hello"}, nil)
	}()

	var events []Event
	for event := range stream.Events() {
		events = append(events, event)
	}

	assert.Len(t, events, 3)
	_, ok := events[0].(EventStart)
	assert.True(t, ok)
	delta, ok := events[1].(EventTextDelta)
	assert.True(t, ok)
	assert.Equal(t, "hello", delta.Delta)
	done, ok := events[2].(EventDone)
	assert.True(t, ok)
	assert.Equal(t, StopReasonStop, done.Reason)

	msg, err := stream.Result()
	require.NoError(t, err)
	assert.Equal(t, "hello", msg.Text)
}

func TestEventStream_ContextCancellation(t *testing.T) {
	// 测试：channel 满时，context 取消后 Push 应该失败
	stream := NewEventStream(1) // buffer=1

	// 先填满 buffer
	err := stream.Push(context.Background(), EventStart{})
	require.NoError(t, err)

	// 现在 buffer 满了，用已取消的 context push 应该失败
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = stream.Push(ctx, EventTextDelta{Delta: "should fail"})
	assert.Error(t, err)
}

func TestEventStream_CloseIdempotent(t *testing.T) {
	stream := NewEventStream(1)
	stream.Close()
	stream.Close() // should not panic

	_, ok := <-stream.Events()
	assert.False(t, ok) // channel should be closed
}

func TestEventStream_ResultBeforeClose(t *testing.T) {
	stream := NewEventStream(1)
	_, err := stream.Result()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not closed")
}
