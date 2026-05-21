package session

import (
	"context"
	"fmt"

	"github.com/earendil-works/pi-go/internal/ai"
)

type Session struct {
	storage SessionStorage
	leafID  string // 当前 leaf entry ID，用于链式追加和恢复
}

func New(storage SessionStorage) *Session {
	return &Session{storage: storage}
}

// Storage returns the underlying storage backend.
func (s *Session) Storage() SessionStorage {
	return s.storage
}

// InitFromStorage 从 storage 中加载当前 leaf，用于恢复已有会话。
func (s *Session) InitFromStorage(ctx context.Context) error {
	leaf, err := s.storage.GetLeaf(ctx)
	if err != nil {
		return err
	}
	s.leafID = leaf
	return nil
}

func (s *Session) AppendMessage(ctx context.Context, msg ai.Message) error {
	entry := Entry{Type: EntryTypeMessage, ParentID: s.leafID}
	switch m := msg.(type) {
	case ai.UserMessage:
		entry.User = &m
	case ai.AssistantMessage:
		entry.Assistant = &m
	case ai.ToolResultMessage:
		entry.Tool = &m
	default:
		return fmt.Errorf("unsupported message type %T", msg)
	}
	if err := s.storage.Append(ctx, entry); err != nil {
		return err
	}
	// 获取 storage 内部分配的 entry ID（Append 已更新 leafID）
	leaf, err := s.storage.GetLeaf(ctx)
	if err != nil {
		return fmt.Errorf("failed to get leaf after append: %w", err)
	}
	// 持久化 leaf 指针，确保重新加载后可以恢复
	if err := s.storage.SetLeaf(ctx, leaf); err != nil {
		return fmt.Errorf("failed to set leaf: %w", err)
	}
	s.leafID = leaf
	return nil
}

func (s *Session) BuildContext(ctx context.Context) ([]ai.Message, error) {
	entries, err := s.storage.GetPathToRoot(ctx, "")
	if err != nil {
		return nil, err
	}
	messages := make([]ai.Message, 0, len(entries))
	for _, entry := range entries {
		switch {
		case entry.User != nil:
			messages = append(messages, *entry.User)
		case entry.Assistant != nil:
			messages = append(messages, *entry.Assistant)
		case entry.Tool != nil:
			messages = append(messages, *entry.Tool)
		}
	}
	return messages, nil
}

func (s *Session) MoveTo(ctx context.Context, entryID string, summary string) error {
	if err := s.storage.SetLeaf(ctx, entryID); err != nil {
		return err
	}
	s.leafID = entryID
	if summary != "" {
		if err := s.storage.Append(ctx, Entry{Type: EntryTypeBranchSummary, Summary: summary, ParentID: s.leafID}); err != nil {
			return err
		}
		// summary entry 追加后，leaf 指向该 entry
		leaf, err := s.storage.GetLeaf(ctx)
		if err != nil {
			return fmt.Errorf("failed to get leaf after summary append: %w", err)
		}
		s.leafID = leaf
	}
	return nil
}
