/**
 * MobileUpdateDialog.tsx — In-app update dialog for Capacitor (Android).
 *
 * Shows when a newer version is available. Allows download + install directly.
 */

import { useState, useEffect } from 'react';
import { Capacitor } from '@capacitor/core';
import { checkMobileUpdate, downloadAndInstallApk, getAppVersion } from '../mobile-updater';
import type { MobileUpdateInfo } from '../types';
import { useT } from '../i18n/useT';

export function MobileUpdateDialog() {
  const [visible, setVisible] = useState(false);
  const [update, setUpdate] = useState<MobileUpdateInfo | null>(null);
  const [phase, setPhase] = useState<'idle' | 'checking' | 'downloading' | 'error'>('idle');
  const [progress, setProgress] = useState(0);
  const [currentVersion, setCurrentVersion] = useState('');
  const [error, setError] = useState('');
  const t = useT();

  // Check for updates on mount (Capacitor only, after 3s delay)
  useEffect(() => {
    if (!Capacitor.isNativePlatform()) return;

    let cancelled = false;
    const timer = setTimeout(async () => {
      try {
        const ver = await getAppVersion();
        if (cancelled) return;
        setCurrentVersion(ver);

        const info = await checkMobileUpdate();
        if (cancelled) return;
        if (info) {
          setUpdate(info);
          setVisible(true);
        }
      } catch (err) {
        // silently fail — don't bother user with update check errors
        console.warn('[MobileUpdateDialog] Update check failed:', err);
      }
    }, 3000);

    // Listen for manual trigger from sidebar button
    const onManualShow = async (e: Event) => {
      const detail = (e as CustomEvent).detail as MobileUpdateInfo | undefined;
      const ver = await getAppVersion();
      if (cancelled) return;
      setCurrentVersion(ver);
      if (detail) {
        setUpdate(detail);
        setVisible(true);
      } else {
        // Re-check
        const info = await checkMobileUpdate();
        if (info) {
          setUpdate(info);
          setVisible(true);
        }
      }
    };
    window.addEventListener('pi-go-show-update', onManualShow);

    return () => {
      cancelled = true;
      clearTimeout(timer);
      window.removeEventListener('pi-go-show-update', onManualShow);
    };
  }, []);

  const handleDownload = async () => {
    if (!update) return;
    setPhase('downloading');
    setProgress(0);
    setError('');
    try {
      await downloadAndInstallApk(update.downloadUrl, (pct) => {
        setProgress(pct);
      });
      // After install intent fires, the app goes to background
      // If user returns without installing, they can retry
      setPhase('idle');
    } catch (err) {
      setPhase('error');
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  const handleSkip = () => {
    setVisible(false);
  };

  if (!visible || !update) return null;

  const sizeMB = update.apkSize ? (update.apkSize / 1024 / 1024).toFixed(1) : '?';

  return (
    <div className="mobile-update-overlay" onClick={(e) => e.stopPropagation()}>
      <div className="mobile-update-card">
        <div className="mobile-update-header">
          <span className="mobile-update-icon">🔄</span>
          <h3>发现新版本 v{update.version}</h3>
        </div>

        <div className="mobile-update-body">
          <p className="mobile-update-current">
            当前版本 v{currentVersion} → 新版本 v{update.version}
          </p>
          <p className="mobile-update-size">
            安装包大小：{sizeMB} MB
          </p>
          {update.releaseNotes && (
            <details className="mobile-update-notes">
              <summary>更新日志</summary>
              <p>{update.releaseNotes.slice(0, 500)}</p>
            </details>
          )}

          {phase === 'downloading' && (
            <div className="mobile-update-progress">
              <div className="mobile-update-bar">
                <div className="mobile-update-bar-fill" style={{ width: `${progress}%` }} />
              </div>
              <span className="mobile-update-percent">{progress}%</span>
            </div>
          )}

          {phase === 'error' && (
            <p className="mobile-update-error">⚠️ {error}</p>
          )}
        </div>

        <div className="mobile-update-actions">
          {phase === 'downloading' ? (
            <button className="mobile-update-btn-primary" disabled>
              下载中... {progress}%
            </button>
          ) : (
            <>
              <button className="mobile-update-btn-secondary" onClick={handleSkip}>
                稍后
              </button>
              <button className="mobile-update-btn-primary" onClick={handleDownload}>
                下载并安装
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
