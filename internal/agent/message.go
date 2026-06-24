package agent

import (
	"sync"

	"github.com/hwj123hwj/pi-go/internal/ai"
)

type MessageQueue struct {
	mu       sync.Mutex
	messages []ai.Message
}

func NewMessageQueue() *MessageQueue {
	return &MessageQueue{messages: make([]ai.Message, 0)}
}

func (q *MessageQueue) Enqueue(msg ai.Message) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.messages = append(q.messages, msg)
}

func (q *MessageQueue) Drain() []ai.Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]ai.Message, len(q.messages))
	copy(out, q.messages)
	q.messages = q.messages[:0]
	return out
}

func (q *MessageQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.messages)
}
