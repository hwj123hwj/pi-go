/// <reference types="vite/client" />

declare module '*.css' {
  const content: string;
  export default content;
}

declare module '*.png' {
  const src: string;
  export default src;
}

declare module '*.webp' {
  const src: string;
  export default src;
}

declare module '*.mp4' {
  const src: string;
  export default src;
}

declare const __APP_VERSION__: string;

// ── window.piAPI type augmentation ──────────────────────────────────────────
interface PiAPI {
  getServerUrl: () => Promise<string | null>;
  startServer: () => Promise<{ url: string; port: number } | { error: string }>;
  checkForUpdate: () => Promise<{
    version: string;
    downloadUrl: string;
    releaseNotes: string;
  } | null>;
  openDownloadPage: (url: string) => Promise<void>;
  pickFolder: () => Promise<string | null>;
  revealInFolder: (path: string) => Promise<void>;
  openInTerminal: (dir: string) => Promise<void>;
  openExternal: (url: string) => Promise<void>;
}

interface Window {
  piAPI?: PiAPI;
}
