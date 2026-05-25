package session

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type JSONLStorage struct {
	path   string
	file   *os.File
	mu     sync.Mutex
	byID   map[string]Entry
	leafID string
}

func NewJSONLStorage(path string) *JSONLStorage {
	return &JSONLStorage{path: path, byID: make(map[string]Entry)}
}

func (s *JSONLStorage) Init() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	if err := s.load(); err != nil {
		return err
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	s.file = file
	return nil
}

// load 将所有 entry 加载到内存中的 byID map。
// 注意：长会话的内存占用会线性增长。MVP 阶段可接受，
// 后续优化可考虑：分页加载、LRU 缓存、或按需加载。
func (s *JSONLStorage) load() error {
	file, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return err
		}
		s.byID[entry.ID] = entry
		if entry.Type == EntryTypeLeaf {
			s.leafID = entry.TargetID
		}
	}
	return scanner.Err()
}

func (s *JSONLStorage) Close() error {
	if s.file == nil {
		return nil
	}
	return s.file.Close()
}

func (s *JSONLStorage) Append(ctx context.Context, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if entry.ID == "" {
		entry.ID = newID("entry")
	}
	if entry.Timestamp == 0 {
		entry.Timestamp = time.Now().UnixMilli()
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := s.file.Write(append(line, '\n')); err != nil {
		return err
	}
	s.byID[entry.ID] = entry
	switch entry.Type {
	case EntryTypeLeaf:
		s.leafID = entry.TargetID
	case EntryTypeMessage, EntryTypeCompaction:
		s.leafID = entry.ID
	}
	return nil
}

func (s *JSONLStorage) GetPathToRoot(ctx context.Context, leafID string) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if leafID == "" {
		leafID = s.leafID
	}
	path := make([]Entry, 0)
	for leafID != "" {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		entry, ok := s.byID[leafID]
		if !ok {
			return nil, fmt.Errorf("entry %q not found", leafID)
		}
		path = append([]Entry{entry}, path...)
		leafID = entry.ParentID
	}
	return path, nil
}

func (s *JSONLStorage) SetLeaf(ctx context.Context, targetID string) error {
	return s.Append(ctx, Entry{ID: newID("leaf"), Type: EntryTypeLeaf, TargetID: targetID})
}

func (s *JSONLStorage) GetLeaf(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.leafID, nil
}

func (s *JSONLStorage) Fork(ctx context.Context, targetID string) (SessionStorage, error) {
	return nil, fmt.Errorf("fork not implemented in MVP")
}

func newID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
