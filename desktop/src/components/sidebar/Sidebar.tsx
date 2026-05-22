// Sidebar.tsx — Sidebar with session list, model selector, and new session button.
import { SessionList } from './SessionList';
import { ModelSelector } from './ModelSelector';
import { NewSessionButton } from './NewSessionButton';
import { useConnectionStore } from '../../stores/connectionStore';
import styles from '../../styles/sidebar.module.css';

export function Sidebar() {
  const connected = useConnectionStore((s) => s.connected);

  return (
    <aside className={styles.sidebar}>
      <div className={styles.header}>
        <div className={styles.logo}>
          <span className={styles.logoIcon}>⚡</span>
          <span className={styles.logoText}>Pi-Go</span>
        </div>
        <div className={`${styles.status} ${connected ? styles.online : styles.offline}`}>
          {connected ? '●' : '○'} {connected ? 'Connected' : 'Disconnected'}
        </div>
      </div>

      <NewSessionButton />
      <SessionList />

      <div className={styles.footer}>
        <ModelSelector />
      </div>
    </aside>
  );
}
