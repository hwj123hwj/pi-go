// preload.ts — Electron preload script for secure IPC bridge.
import { contextBridge, ipcRenderer } from 'electron';

export interface PiAPI {
  getServerUrl: () => Promise<string | null>;
  startServer: () => Promise<{ url: string; port: number } | { error: string }>;
}

contextBridge.exposeInMainWorld('piAPI', {
  getServerUrl: () => ipcRenderer.invoke('get-server-url'),
  startServer: () => ipcRenderer.invoke('start-server'),
});
