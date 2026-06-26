/**
 * ServerConnect.tsx — Shown on mobile/PWA platforms to configure the remote
 * pi-go backend URL.
 *
 * On Electron, the backend runs locally and its URL is obtained via IPC.
 * On mobile (Capacitor) or browser, the user must point to a remote server.
 */

import { useState } from 'react';
import { getStoredServerUrl, setStoredServerUrl } from '../platform';
import { setBaseUrl } from '../store';
import { Icon } from './Icon';
import { useT } from '../i18n/useT';

export function ServerConnect({ onConnect }: { onConnect: () => void }) {
  const t = useT();
  const saved = getStoredServerUrl() || '';
  const [url, setUrl] = useState(saved);
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleConnect = async () => {
    const trimmed = url.trim().replace(/\/+$/, '');
    if (!trimmed) {
      setError(t('server.errorEmpty'));
      return;
    }

    // Basic validation
    if (!/^https?:\/\/.+/.test(trimmed)) {
      setError(t('server.errorFormat'));
      return;
    }

    setTesting(true);
    setError(null);

    try {
      // Health check
      const res = await fetch(`${trimmed}/health`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);

      // Success — persist and proceed
      setStoredServerUrl(trimmed);
      setBaseUrl(trimmed);
      onConnect();
    } catch (err) {
      setError(`${t('server.errorConnect')}: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setTesting(false);
    }
  };

  return (
    <div className="server-connect">
      <div className="server-connect-card">
        <div className="server-connect-icon">
          <Icon name="sparkle" size={28} />
        </div>
        <h1>Pi-Go</h1>
        <p className="server-connect-subtitle">{t('server.subtitle')}</p>

        <div className="server-connect-input">
          <Icon name="link" size={16} />
          <input
            type="url"
            value={url}
            placeholder="http://192.168.1.100:8080"
            autoFocus
            onChange={(e) => setUrl(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleConnect()}
          />
        </div>

        {error && <div className="server-connect-error">{error}</div>}

        <button
          className="server-connect-btn"
          disabled={testing}
          onClick={handleConnect}
        >
          {testing ? (
            <>
              <Icon name="loader" size={16} spin />
              {t('server.connecting')}
            </>
          ) : (
            <>
              <Icon name="arrow-right" size={16} />
              {t('server.connect')}
            </>
          )}
        </button>

        <div className="server-connect-hint">
          <Icon name="info" size={13} />
          <span>{t('server.hint')}</span>
        </div>
      </div>
    </div>
  );
}
