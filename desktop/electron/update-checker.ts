// update-checker.ts — Checks GitHub Releases for new versions.
// Uses GitHub API + simple version comparison. Opens browser for download
// since the app is unsigned and electron-updater auto-install won't work.
import { app, net } from 'electron';

const REPO = 'hwj123hwj/pi-go';

export interface UpdateInfo {
  version: string;
  downloadUrl: string;
  releaseNotes: string;
}

// compareVersions returns true if `latest` is newer than `current`.
// Both are expected to be dot-separated version strings like "0.3.0".
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

// checkForUpdate polls GitHub Releases API and returns update info
// if a newer version exists, or null if already up-to-date.
export async function checkForUpdate(): Promise<UpdateInfo | null> {
  const currentVersion = app.getVersion();

  try {
    const res = await net.fetch(`https://api.github.com/repos/${REPO}/releases/latest`);
    if (!res.ok) {
      console.warn(`[update-checker] GitHub API returned ${res.status}`);
      return null;
    }

    const release = await res.json();
    if (!release.tag_name) {
      return null;
    }

    const latestVersion = release.tag_name.replace(/^v/, '');

    if (!isNewer(latestVersion, currentVersion)) {
      return null;
    }

    // Find the arm64 DMG asset
    const dmgAsset = (release.assets || []).find(
      (a: any) => typeof a.name === 'string' && a.name.includes('-arm64.dmg')
    );

    return {
      version: latestVersion,
      downloadUrl: dmgAsset?.browser_download_url || release.html_url,
      releaseNotes: release.body || '',
    };
  } catch (err) {
    console.warn('[update-checker] Failed to check for updates:', err);
    return null;
  }
}
