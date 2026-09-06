package tools

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hwj123hwj/pi-go/sdk/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileMutationQueue_SameFileSerialized(t *testing.T) {
	q := NewFileMutationQueue()
	ctx := context.Background()

	var order []int
	var mu sync.Mutex
	counter := int32(0)

	// 启动 3 个对同一文件的操作
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := q.Execute(ctx, "/tmp/same_file.go", func() (agent.ToolResult, error) {
				n := atomic.AddInt32(&counter, 1)
				mu.Lock()
				order = append(order, int(n))
				mu.Unlock()
				time.Sleep(10 * time.Millisecond) // 模拟耗时操作
				return agent.ToolResult{}, nil
			})
			require.NoError(t, err)
		}(i)
	}
	wg.Wait()

	// 验证串行执行：order 应该是 [1, 2, 3]
	assert.Equal(t, []int{1, 2, 3}, order)
}

func TestFileMutationQueue_DifferentFilesParallel(t *testing.T) {
	q := NewFileMutationQueue()
	ctx := context.Background()

	var running atomic.Int32
	var maxRunning atomic.Int32

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := q.Execute(ctx, "/tmp/file_"+string(rune('a'+idx))+".go", func() (agent.ToolResult, error) {
				r := running.Add(1)
				for {
					cur := maxRunning.Load()
					if r <= cur || maxRunning.CompareAndSwap(cur, r) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				running.Add(-1)
				return agent.ToolResult{}, nil
			})
			require.NoError(t, err)
		}(i)
	}
	wg.Wait()

	// 不同文件应该并行执行过
	assert.Greater(t, maxRunning.Load(), int32(1), "different files should run in parallel")
}

func TestFileMutationQueue_ContextCancel(t *testing.T) {
	q := NewFileMutationQueue()
	ctx, cancel := context.WithCancel(context.Background())

	// 第一个操作阻塞
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		q.Execute(ctx, "/tmp/blocking.go", func() (agent.ToolResult, error) {
			time.Sleep(200 * time.Millisecond)
			return agent.ToolResult{}, nil
		})
	}()

	// 等第一个操作开始
	time.Sleep(20 * time.Millisecond)

	// 第二个操作，然后 cancel context
	cancel()
	_, err := q.Execute(ctx, "/tmp/blocking.go", func() (agent.ToolResult, error) {
		return agent.ToolResult{}, nil
	})
	assert.Error(t, err)
	wg.Wait()
}
