// update-checker.ts — Checks GitHub Releases for new versions.
// Uses GitHub API + semver comparison. Opens browser for download
// since the app is unsigned and electron-updater auto-install won't work.
import { app, net } from 'electron';
import * as semver from 'semver';

const REPO = 'hwj123hwj/pi-go';

export interface UpdateInfo {
  version: string;
  downloadUrl: string;
  releaseNotes: string;
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

    if (!semver.gt(latestVersion, currentVersion)) {
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
