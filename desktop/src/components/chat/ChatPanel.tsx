// ChatPanel.tsx — Main chat area: message list + input.
import { useEffect, useMemo, useRef } from 'react';
import { useChatStore } from '../../stores/chatStore';
import { useSessionStore } from '../../stores/sessionStore';
import { MessageList } from './MessageList';
import { InputArea } from '../input/InputArea';
import styles from '../../styles/chat.module.css';

const EMPTY_MESSAGES: any[] = [];

export function ChatPanel() {
  const currentSessionId = useSessionStore((s) => s.currentSessionId);
  const messagesMap = useChatStore((s) => s.messagesBySession);
  const streamingSessionId = useChatStore((s) => s.streamingSessionId);
  const sendPrompt = useChatStore((s) => s.sendPrompt);
  const cancelGeneration = useChatStore((s) => s.cancelGeneration);
  const loadHistory = useChatStore((s) => s.loadHistory);
  // Track which sessions have already been loaded
  const loadedRef = useRef<Set<string>>(new Set());

  // Load history when switching to a session that hasn't been loaded yet
  useEffect(() => {
    if (currentSessionId && !loadedRef.current.has(currentSessionId)) {
      loadedRef.current.add(currentSessionId);
      loadHistory(currentSessionId);
    }
  }, [currentSessionId, loadHistory]);

  // Use stable reference — return same EMPTY_MESSAGES when no messages
  const messages = useMemo(
    () => messagesMap[currentSessionId] || EMPTY_MESSAGES,
    [messagesMap, currentSessionId]
  );
  const isStreaming = streamingSessionId === currentSessionId;

  if (!currentSessionId) {
    return (
      <div className={styles.empty}>
        <div className={styles.emptyIcon}>💬</div>
        <div className={styles.emptyTitle}>Welcome to Pi-Go</div>
        <div className={styles.emptySubtitle}>
          Create a new session or select an existing one to start chatting.
        </div>
      </div>
    );
  }

  return (
    <div className={styles.chatPanel}>
      <MessageList messages={messages} />
      <InputArea
        onSend={(text) => sendPrompt(currentSessionId, text)}
        onCancel={cancelGeneration}
        isStreaming={isStreaming}
      />
    </div>
  );
}
