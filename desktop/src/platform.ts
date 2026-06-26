/**
 * platform.ts — Runtime platform detection.
 *
 * Three platforms share the same React frontend:
 * 1. Electron (desktop) — has `window.piAPI` IPC bridge, backend runs locally
 * 2. Capacitor (mobile) — no IPC, backend runs on a remote server
 * 3. Browser (PWA/preview) — no IPC, backend runs on a configured server
 *
 * The platform affects:
 * - How the server URL is obtained (IPC vs localStorage)
 * - Whether folder picking is available (Electron only)
 * - Whether update checks are available (Electron only)
 * - Layout: mobile platforms get drawer-style sidebars
 */

/** true when running inside Electron (window.piAPI exists) */
export const isElectron = typeof window !== 'undefined' && !!window.piAPI;

/** true when running inside Capacitor (Android/iOS webview) */
export const isCapacitor =
  typeof window !== 'undefined' &&
  typeof (window as any).Capacitor !== 'undefined' &&
  (window as any).Capacitor?.isNativePlatform?.();

/** true on small touch screens (phone form factor) */
export const isMobileViewport = (): boolean => {
  if (typeof window === 'undefined') return false;
  return window.matchMedia('(max-width: 768px)').matches;
};

/** true when the platform uses a remote server (Capacitor or browser) */
export const isRemotePlatform = !isElectron;

/** Whether folder picking (native dialog) is available */
export const canPickFolder = isElectron;

/** Whether app update checks are available */
export const canCheckUpdates = isElectron;

// ── Mobile server URL persistence ──────────────────────────────────────────

const STORAGE_KEY = 'pi-go-server-url';

export function getStoredServerUrl(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}

export function setStoredServerUrl(url: string): void {
  try {
    localStorage.setItem(STORAGE_KEY, url);
  } catch {
    // ignore
  }
}
