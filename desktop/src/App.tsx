import { useEffect } from 'react';
import { useStore } from './store';
import { Sidebar } from './components/Sidebar';
import { SessionView } from './components/SessionView';
import { UpdateBanner } from './components/UpdateBanner';
import { ErrorBoundary } from './components/ErrorBoundary';
import { Icon } from './components/Icon';
import { useT } from './i18n/useT';
import { applyTheme } from './theme';

export function App() {
  const init = useStore((s) => s.init);
  const ready = useStore((s) => s.ready);
  const lang = useStore((s) => s.lang);
  const theme = useStore((s) => s.theme);
  const t = useT();

  useEffect(() => {
    void init();
  }, [init]);

  useEffect(() => {
    document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en';
  }, [lang]);

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  if (!ready) {
    return (
      <div className="boot">
        <div className="boot-inner">
          <span className="brand-mark" style={{ width: 40, height: 40, borderRadius: 12 }}>
            <Icon name="sparkle" size={20} />
          </span>
          <span className="spinner" />
          <span>{t('app.booting')}</span>
        </div>
      </div>
    );
  }

  return (
    <ErrorBoundary label="app">
      <div className="app">
        <Sidebar />
        <ErrorBoundary label="session">
          <SessionView />
        </ErrorBoundary>
        <UpdateBanner />
      </div>
    </ErrorBoundary>
  );
}
