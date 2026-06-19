// preload.ts — Electron preload script for secure IPC bridge.
import { contextBridge, ipcRenderer } from 'electron';

export interface UpdateInfo {
  version: string;
  downloadUrl: string;
  releaseNotes: string;
}

export interface PiAPI {
  getServerUrl: () => Promise<string | null>;
  startServer: () => Promise<{ url: string; port: number } | { error: string }>;
  checkForUpdate: () => Promise<UpdateInfo | null>;
  openDownloadPage: (url: string) => Promise<void>;
  pickFolder: () => Promise<string | null>;
}

contextBridge.exposeInMainWorld('piAPI', {
  getServerUrl: () => ipcRenderer.invoke('get-server-url'),
  startServer: () => ipcRenderer.invoke('start-server'),
  checkForUpdate: () => ipcRenderer.invoke('check-for-update'),
  openDownloadPage: (url: string) => ipcRenderer.invoke('open-download-page', url),
  pickFolder: () => ipcRenderer.invoke('pick-folder'),
});
