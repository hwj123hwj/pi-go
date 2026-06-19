import React from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import './styles/app.css';

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);

declare global {
  interface Window {
    piAPI?: {
      getServerUrl: () => Promise<string | null>;
      startServer: () => Promise<{ url: string; port: number } | { error: string }>;
      checkForUpdate: () => Promise<{
        version: string;
        downloadUrl: string;
        releaseNotes: string;
      } | null>;
      openDownloadPage: (url: string) => Promise<void>;
      pickFolder: () => Promise<string | null>;
    };
  }
}
