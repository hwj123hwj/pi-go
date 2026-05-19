package session

import (
	"context"
	"fmt"

	"github.com/earendil-works/pi-go/internal/ai"
)

type Session struct {
	storage SessionStorage
}

func New(storage SessionStorage) *Session {
	return &Session{storage: storage}
}

func (s *Session) AppendMessage(ctx context.Context, msg ai.Message) error {
	entry := Entry{Type: EntryTypeMessage}
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
	return s.storage.Append(ctx, entry)
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
	if summary != "" {
		return s.storage.Append(ctx, Entry{Type: EntryTypeBranchSummary, Summary: summary})
	}
	return nil
}
