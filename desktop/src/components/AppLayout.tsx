// AppLayout.tsx — Main layout: sidebar + chat panel.
import { Sidebar } from './sidebar/Sidebar';
import { ChatPanel } from './chat/ChatPanel';
import { UpdateNotification } from './common/UpdateNotification';
import styles from '../styles/layout.module.css';

export function AppLayout() {
  return (
    <>
      <UpdateNotification />
      <div className={styles.layout}>
        <Sidebar />
        <main className={styles.main}>
          <ChatPanel />
        </main>
      </div>
    </>
  );
}
