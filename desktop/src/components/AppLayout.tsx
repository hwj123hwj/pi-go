// AppLayout.tsx — Main layout: sidebar + chat panel.
import { Sidebar } from './sidebar/Sidebar';
import { ChatPanel } from './chat/ChatPanel';
import styles from '../styles/layout.module.css';

export function AppLayout() {
  return (
    <div className={styles.layout}>
      <Sidebar />
      <main className={styles.main}>
        <ChatPanel />
      </main>
    </div>
  );
}
