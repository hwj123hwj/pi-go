/**
 * mobile-updater.ts — In-app self-update for Android (Capacitor).
 *
 * Uses a custom Capacitor plugin (ApkUpdaterPlugin) to:
 * 1. Check GitHub Releases for latest version
 * 2. Compare semver with current app version
 * 3. Download APK natively (with progress events) and trigger system installer
 *
 * Requires:
 * - android.permission.REQUEST_INSTALL_PACKAGES in AndroidManifest.xml
 * - FileProvider configured with apk_downloads path
 * - ApkUpdaterPlugin.java registered in MainActivity
 */

import { Capacitor } from '@capacitor/core';
import type { MobileUpdateInfo } from './types';

const REPO = 'hwj123hwj/pi-go';

/** Custom Capacitor plugin registration */
interface ApkUpdaterPlugin {
  getAppVersion(): Promise<{ version: string }>;
  downloadAndInstall(options: { url: string; fileName?: string }): Promise<{ success: boolean; path: string }>;
  addListener(eventName: string, listener: (data: { percent: number }) => void): Promise<{ remove: () => void }>;
}

const ApkUpdater = (Capacitor as any).registerPlugin('ApkUpdater') as ApkUpdaterPlugin;

/** Get current app version (from native). */
export async function getAppVersion(): Promise<string> {
  try {
    const { version } = await ApkUpdater.getAppVersion();
    return version;
  } catch {
    return '0.0.0';
  }
}

/** Compare semver: returns true if `latest` is newer than `current`. */
function isNewer(latest: string, current: string): boolean {
  const a = latest.split('.').map((n) => parseInt(n, 10) || 0);
  const b = current.split('.').map((n) => parseInt(n, 10) || 0);
  for (let i = 0; i < Math.max(a.length, b.length); i++) {
    const av = a[i] || 0;
    const bv = b[i] || 0;
    if (av > bv) return true;
    if (av < bv) return false;
  }
  return false;
}

/**
 * Check GitHub Releases for a newer APK version.
 * Returns null if already up-to-date.
 */
export async function checkMobileUpdate(): Promise<MobileUpdateInfo | null> {
  try {
    const res = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`);
    if (!res.ok) return null;

    const release = await res.json();
    if (!release.tag_name) return null;

    const latestVersion = release.tag_name.replace(/^v/, '');
    const currentVersion = await getAppVersion();

    if (!isNewer(latestVersion, currentVersion)) {
      return null;
    }

    // Find the APK asset
    const apkAsset = (release.assets || []).find(
      (a: any) => a.name === 'pi-go-debug.apk' || (typeof a.name === 'string' && a.name.endsWith('.apk'))
    );

    if (!apkAsset) return null;

    return {
      version: latestVersion,
      downloadUrl: apkAsset.browser_download_url,
      releaseNotes: release.body || '',
      apkSize: apkAsset.size || 0,
    };
  } catch (err) {
    console.warn('[mobile-updater] Failed to check for updates:', err);
    return null;
  }
}

/**
 * Download APK and trigger system installer (Android only).
 * onProgress callback receives download percentage (0-100).
 */
export async function downloadAndInstallApk(
  downloadUrl: string,
  onProgress?: (percent: number) => void,
): Promise<void> {
  // Listen for native progress events
  let progressListener: { remove: () => void } | null = null;
  try {
    if (ApkUpdater?.addListener) {
      progressListener = await ApkUpdater.addListener('downloadProgress', (data: { percent: number }) => {
        if (onProgress) onProgress(data.percent);
      });
    }
  } catch {
    // listener may not be available on all platforms
  }

  try {
    await ApkUpdater.downloadAndInstall({
      url: downloadUrl,
      fileName: 'pi-go-update.apk',
    });
  } finally {
    progressListener?.remove();
  }
}
