package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		err    string
		expect bool
	}{
		{"API error 500: internal server error", true},
		{"API error 502: bad gateway", true},
		{"API error 503: service unavailable", true},
		{"API error 429: rate limit exceeded", true},
		{"connection reset by peer", true},
		{"timeout waiting for response", true},
		{"unexpected EOF", true},
		{"API error 400: bad request", false},
		{"invalid api key", false},
		{"context canceled", false},
		{"context deadline exceeded", false},
		{"", false},
	}

	for _, tc := range tests {
		result := IsRetryableError(errors.New(tc.err))
		assert.Equal(t, tc.expect, result, "IsRetryableError(%q) = %v, want %v", tc.err, result, tc.expect)
	}
}

func TestIsRetryableError_Nil(t *testing.T) {
	assert.False(t, IsRetryableError(nil))
}

func TestStreamWithRetry_Success(t *testing.T) {
	config := DefaultRetryConfig()
	config.MaxRetries = 2

	callCount := 0
	fn := func(ctx context.Context) (*EventStream, error) {
		callCount++
		stream := NewEventStream(4)
		go func() {
			defer stream.Close()
			partial := StreamAssistantMessage{Text: "hello"}
			_ = stream.Push(ctx, EventStart{Partial: partial})
			_ = stream.Push(ctx, EventDone{Reason: StopReasonStop, Message: partial})
			stream.SetResult(partial, nil)
		}()
		return stream, nil
	}

	stream, err := StreamWithRetry(context.Background(), config, fn)
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	assert.Equal(t, 1, callCount) // 不重试成功的情况
}

func TestStreamWithRetry_NonRetryableError(t *testing.T) {
	config := DefaultRetryConfig()
	config.MaxRetries = 3

	callCount := 0
	fn := func(ctx context.Context) (*EventStream, error) {
		callCount++
		return nil, errors.New("invalid api key")
	}

	_, err := StreamWithRetry(context.Background(), config, fn)
	assert.Error(t, err)
	assert.Equal(t, 1, callCount) // 不重试不可重试的错误
}

func TestStreamWithRetry_ContextCancelled(t *testing.T) {
	config := DefaultRetryConfig()
	config.MaxRetries = 3
	config.InitialBackoff = 0 // 快速重试

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fn := func(ctx context.Context) (*EventStream, error) {
		return nil, errors.New("API error 500: internal")
	}

	_, err := StreamWithRetry(ctx, config, fn)
	assert.Error(t, err)
}
