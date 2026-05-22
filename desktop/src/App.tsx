// App.tsx — Root component with initialization logic.
import { useEffect, useState } from 'react';
import { useConnectionStore } from './stores/connectionStore';
import { useSessionStore } from './stores/sessionStore';
import { useModelStore } from './stores/modelStore';
import { useChatStore } from './stores/chatStore';
import { useUpdateStore } from './stores/updateStore';
import { setBaseUrl } from './services/api';
import { AppLayout } from './components/AppLayout';
import { ErrorBanner } from './components/common/ErrorBanner';

export default function App() {
  const [initState, setInitState] = useState<'loading' | 'ready' | 'error'>('loading');
  const [initError, setInitError] = useState('');
  const connect = useConnectionStore((s) => s.connect);
  const loadSessions = useSessionStore((s) => s.loadSessions);
  const loadModels = useModelStore((s) => s.loadModels);
  const setupWSListeners = useChatStore((s) => s.setupWSListeners);

  useEffect(() => {
    async function init() {
      try {
        let serverUrl: string;

        // Check if running in Electron (has window.piAPI)
        if (window.piAPI) {
          const result = await window.piAPI.startServer();
          if ('error' in result) {
            throw new Error(result.error);
          }
          serverUrl = result.url;
        } else {
          // Running in browser (development mode) — connect to localhost
          serverUrl = 'http://127.0.0.1:8080';
        }

        setBaseUrl(serverUrl);

        // Setup WebSocket listeners before connecting
        setupWSListeners();

        // Connect WebSocket
        connect(serverUrl);

        // Load initial data
        await Promise.all([loadSessions(), loadModels()]);

        setInitState('ready');
      } catch (err: any) {
        console.error('Init failed', err);
        setInitError(err.message || 'Failed to initialize');
        setInitState('error');
      }
    }

    init();
  }, []);

  // Check for updates after app is ready (non-blocking)
  useEffect(() => {
    if (initState === 'ready') {
      useUpdateStore.getState().checkForUpdate();
    }
  }, [initState]);

  if (initState === 'loading') {
    return (
      <div style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100vh',
        background: 'var(--bg-primary)',
        color: 'var(--text-secondary)',
        fontFamily: 'sans-serif',
      }}>
        <div style={{ textAlign: 'center' }}>
          <div style={{ fontSize: 48, marginBottom: 16 }}>⚡</div>
          <div>Starting Pi-Go...</div>
        </div>
      </div>
    );
  }

  if (initState === 'error') {
    return <ErrorBanner message={initError} />;
  }

  return <AppLayout />;
}
