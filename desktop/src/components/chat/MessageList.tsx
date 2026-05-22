// MessageList.tsx — Scrollable message list with auto-scroll.
import { useEffect, useRef } from 'react';
import { Message } from '../../stores/chatStore';
import { UserMessage } from './UserMessage';
import { AssistantMessage } from './AssistantMessage';
import styles from '../../styles/chat.module.css';

interface Props {
  messages: Message[];
}

export function MessageList({ messages }: Props) {
  const bottomRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    if (bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [messages]);

  return (
    <div className={styles.messageList} ref={containerRef}>
      {messages.map((msg) =>
        msg.role === 'user' ? (
          <UserMessage key={msg.id} message={msg} />
        ) : (
          <AssistantMessage key={msg.id} message={msg} />
        )
      )}
      <div ref={bottomRef} />
    </div>
  );
}
