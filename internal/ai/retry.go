package ai

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// RetryConfig 控制 LLM 请求的重试行为。
type RetryConfig struct {
	// 最大重试次数（不包括首次请求）
	MaxRetries int
	// 初始退避时间
	InitialBackoff time.Duration
	// 最大退避时间
	MaxBackoff time.Duration
	// 退避倍数
	BackoffMultiplier float64
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// IsRetryableError 判断错误是否可以重试。
// 可重试的错误包括：
//   - 5xx 服务器错误
//   - 429 速率限制
//   - 网络超时
//   - 连接重置
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()

	// 5xx 错误
	if containsAny(msg, "500", "502", "503", "504") {
		return true
	}

	// 速率限制
	if containsAny(msg, "429", "rate limit", "rate_limit", "too many requests") {
		return true
	}

	// 网络错误
	if containsAny(msg, "timeout", "connection reset", "EOF", "temporary") {
		return true
	}

	// 上下文取消/超时不重试
	if containsAny(msg, "context canceled", "context deadline exceeded") {
		return false
	}

	return false
}

// RetryableStreamFunc 是可以重试的流式请求函数。
type RetryableStreamFunc func(ctx context.Context) (*EventStream, error)

// StreamWithRetry 执行带重试的流式请求。
// 消费者从返回的 EventStream 读取事件。
// 如果所有重试都失败，返回最后一个错误。
func StreamWithRetry(ctx context.Context, config RetryConfig, fn RetryableStreamFunc) (*EventStream, error) {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := calculateBackoff(config, attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		stream, err := fn(ctx)
		if err == nil {
			return stream, nil
		}

		lastErr = err

		// 检查是否可以重试
		if !IsRetryableError(err) {
			return nil, err
		}

		// 检查上下文是否已取消
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("all %d retries exhausted: %w", config.MaxRetries, lastErr)
}

func calculateBackoff(config RetryConfig, attempt int) time.Duration {
	backoff := time.Duration(float64(config.InitialBackoff) * math.Pow(config.BackoffMultiplier, float64(attempt-1)))
	if backoff > config.MaxBackoff {
		backoff = config.MaxBackoff
	}
	return backoff
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
