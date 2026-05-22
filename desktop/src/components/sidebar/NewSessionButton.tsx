// NewSessionButton.tsx — Button to create a new chat session.
import { useSessionStore } from '../../stores/sessionStore';
import { useChatStore } from '../../stores/chatStore';
import styles from '../../styles/sidebar.module.css';

export function NewSessionButton() {
  const createSession = useSessionStore((s) => s.createSession);
  const loadHistory = useChatStore((s) => s.loadHistory);

  const handleClick = async () => {
    const id = await createSession();
    // Initialize empty messages for new session
    loadHistory(id);
  };

  return (
    <button className={styles.newSessionButton} onClick={handleClick}>
      + New Chat
    </button>
  );
}
