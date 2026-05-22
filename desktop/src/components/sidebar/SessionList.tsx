// SessionList.tsx — List of chat sessions with switch/delete.
import { useSessionStore, Session } from '../../stores/sessionStore';
import { useChatStore } from '../../stores/chatStore';
import styles from '../../styles/sidebar.module.css';

export function SessionList() {
  const sessions = useSessionStore((s) => s.sessions);
  const currentSessionId = useSessionStore((s) => s.currentSessionId);
  const switchSession = useSessionStore((s) => s.switchSession);
  const deleteSession = useSessionStore((s) => s.deleteSession);
  const streamingSessionId = useChatStore((s) => s.streamingSessionId);

  const handleSwitch = (id: string) => {
    if (id !== currentSessionId) {
      switchSession(id);
    }
  };

  const handleDelete = (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    deleteSession(id);
  };

  if (sessions.length === 0) {
    return (
      <div className={styles.emptySessions}>
        No sessions yet. Create one to get started!
      </div>
    );
  }

  return (
    <div className={styles.sessionList}>
      {sessions.map((session) => (
        <div
          key={session.id}
          className={`${styles.sessionItem} ${session.id === currentSessionId ? styles.active : ''}`}
          onClick={() => handleSwitch(session.id)}
        >
          <div className={styles.sessionInfo}>
            <div className={styles.sessionTitle}>
              {session.id.slice(0, 16)}...
              {session.id === streamingSessionId && (
                <span className={styles.streamingDot}>●</span>
              )}
            </div>
            <div className={styles.sessionMeta}>
              {session.messageCount > 0 ? `${session.messageCount} messages` : 'New'}
            </div>
          </div>
          <button
            className={styles.deleteButton}
            onClick={(e) => handleDelete(e, session.id)}
            title="Delete session"
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
