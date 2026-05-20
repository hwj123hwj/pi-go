package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONLStorage_AppendAndGetPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	storage := NewJSONLStorage(path)
	require.NoError(t, storage.Init())
	defer storage.Close()

	ctx := context.Background()

	// 追加 user message（第一条 entry，无 parent）
	userMsg := ai.NewTextUserMessage("hello")
	entry1 := Entry{Type: EntryTypeMessage, User: &userMsg}
	require.NoError(t, storage.Append(ctx, entry1))
	entry1ID := ""
	for id := range storage.byID {
		entry1ID = id
	}

	// 追加 assistant message（parent 指向 user message）
	assistantMsg := ai.AssistantMessage{Text: "world"}
	require.NoError(t, storage.Append(ctx, Entry{Type: EntryTypeMessage, Assistant: &assistantMsg, ParentID: entry1ID}))
	var entry2ID string
	for id, e := range storage.byID {
		if e.Assistant != nil {
			entry2ID = id
		}
	}

	// 设置 leaf 指向 assistant message
	require.NoError(t, storage.SetLeaf(ctx, entry2ID))

	// 获取路径 root → leaf
	pathEntries, err := storage.GetPathToRoot(ctx, "")
	require.NoError(t, err)
	assert.Len(t, pathEntries, 2)

	// 第一条是 user message
	assert.Equal(t, EntryTypeMessage, pathEntries[0].Type)
	assert.NotNil(t, pathEntries[0].User)
	assert.Equal(t, "hello", pathEntries[0].User.Content[0].Text)

	// 第二条是 assistant message
	assert.NotNil(t, pathEntries[1].Assistant)
	assert.Equal(t, "world", pathEntries[1].Assistant.Text)
}

func TestJSONLStorage_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	ctx := context.Background()

	// 写入数据
	storage1 := NewJSONLStorage(path)
	require.NoError(t, storage1.Init())
	userMsg := ai.NewTextUserMessage("test message")
	require.NoError(t, storage1.Append(ctx, Entry{Type: EntryTypeMessage, User: &userMsg}))
	var entry1ID string
	for id := range storage1.byID {
		entry1ID = id
	}

	assistantMsg := ai.AssistantMessage{Text: "test response"}
	require.NoError(t, storage1.Append(ctx, Entry{Type: EntryTypeMessage, Assistant: &assistantMsg, ParentID: entry1ID}))
	var entry2ID string
	for id, e := range storage1.byID {
		if e.Assistant != nil {
			entry2ID = id
		}
	}
	require.NoError(t, storage1.SetLeaf(ctx, entry2ID))
	require.NoError(t, storage1.Close())

	// 重新加载
	storage2 := NewJSONLStorage(path)
	require.NoError(t, storage2.Init())
	defer storage2.Close()

	pathEntries, err := storage2.GetPathToRoot(ctx, "")
	require.NoError(t, err)
	assert.Len(t, pathEntries, 2)
	assert.Equal(t, "test message", pathEntries[0].User.Content[0].Text)
	assert.Equal(t, "test response", pathEntries[1].Assistant.Text)
}

func TestJSONLStorage_GetPathToRoot_NotFound(t *testing.T) {
	dir := t.TempDir()
	storage := NewJSONLStorage(filepath.Join(dir, "test.jsonl"))
	require.NoError(t, storage.Init())
	defer storage.Close()

	_, err := storage.GetPathToRoot(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestJSONLStorage_InitCreatesDir(t *testing.T) {
	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "sub", "dir", "session.jsonl")
	storage := NewJSONLStorage(nestedPath)
	require.NoError(t, storage.Init())
	defer storage.Close()

	_, err := os.Stat(filepath.Join(dir, "sub", "dir"))
	assert.NoError(t, err)
}
