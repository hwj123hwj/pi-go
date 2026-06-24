package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/hwj123hwj/pi-go/internal/agent"
)

// FileMutationQueue 提供 per-file 的 FIFO 串行化队列。
// 用于保护同一文件的并发写操作，不同文件的写操作可并行。
//
// 使用方式：
//
//	result, err := queue.Execute(ctx, filePath, func() (agent.ToolResult, error) {
//	    // read-modify-write 操作
//	})
//
// Mutation key 是 filePath（规范化后的绝对路径，文件级粒度）。
type FileMutationQueue struct {
	mu     sync.Mutex
	queues map[string]*queueEntry
}

type queueEntry struct {
	ch chan struct{} // 当前操作完成后关闭
}

// NewFileMutationQueue 创建一个新的 FileMutationQueue。
func NewFileMutationQueue() *FileMutationQueue {
	return &FileMutationQueue{
		queues: make(map[string]*queueEntry),
	}
}

// Execute 在指定文件的 mutation queue 中执行 fn。
// 如果同一文件有正在执行的操作，会等待其完成后再执行。
// 不同文件的调用可并行。
//
// filePath 会被 filepath.EvalSymlinks 规范化（如果文件已存在），
// 以确保不同路径指向同一文件时共享同一个 queue entry。
func (q *FileMutationQueue) Execute(ctx context.Context, filePath string, fn func() (agent.ToolResult, error)) (agent.ToolResult, error) {
	key, err := q.normalizePath(filePath)
	if err != nil {
		// 规范化失败（如文件不存在但即将创建）——用原始路径作为 key
		key = filepath.Clean(filePath)
	}

	// 原子地获取前一个 entry 并注册当前 entry。
	q.mu.Lock()
	prev := q.queues[key]
	current := &queueEntry{ch: make(chan struct{})}
	q.queues[key] = current
	q.mu.Unlock()

	// defer close: 确保 fn panic 时 current.ch 也被关闭，
	// 后续等待的调用不会死锁。
	shouldClose := true
	defer func() {
		if shouldClose {
			close(current.ch)
		}
	}()

	// 等待前一个操作完成（如果有）
	if prev != nil {
		select {
		case <-prev.ch:
			// 前一个操作完成，继续执行
		case <-ctx.Done():
			// Context 取消：不执行 fn，也不 close current.ch。
			// 恢复 prev 为 map entry，保持 FIFO 链不断——
			// 后续调用继续等待 prev 完成。
			q.mu.Lock()
			q.queues[key] = prev
			q.mu.Unlock()
			shouldClose = false
			return agent.ToolResult{IsError: true, Content: ctx.Err().Error()}, ctx.Err()
		}
	}

	// 执行操作（fn 可能 panic，但 defer 确保 close）
	result, err := fn()

	// 清理：如果当前 entry 仍是最新（没有后续等待者），删除 map entry
	q.mu.Lock()
	if entry, ok := q.queues[key]; ok && entry == current {
		delete(q.queues, key)
	}
	q.mu.Unlock()

	return result, err
}

// normalizePath 尝试规范化路径。如果文件不存在则返回原始路径和错误。
func (q *FileMutationQueue) normalizePath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path, fmt.Errorf("normalize path: %w", err)
	}
	return resolved, nil
}
