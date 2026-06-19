// main.ts — Electron main process for pi-go desktop client.
import { app, BrowserWindow, ipcMain, shell, dialog } from 'electron';
import * as path from 'path';
import { PiGoManager } from './pi-go-manager';
import { checkForUpdate } from './update-checker';

let mainWindow: BrowserWindow | null = null;
const piGoManager = new PiGoManager();

async function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    minWidth: 800,
    minHeight: 600,
    title: 'Pi-Go',
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  // In development, load from Vite dev server
  const isDev = !app.isPackaged;
  if (isDev) {
    await mainWindow.loadURL('http://localhost:5173');
    mainWindow.webContents.openDevTools();
  } else {
    await mainWindow.loadFile(path.join(__dirname, '..', 'renderer', 'index.html'));
  }

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

// IPC handlers
ipcMain.handle('get-server-url', () => {
  const info = piGoManager.getServerInfo();
  return info ? info.url : null;
});

ipcMain.handle('start-server', async () => {
  // If already started, return existing info
  const existing = piGoManager.getServerInfo();
  if (existing) {
    return { url: existing.url, port: existing.port };
  }
  try {
    const info = await piGoManager.start();
    return { url: info.url, port: info.port };
  } catch (err: any) {
    return { error: err.message };
  }
});

// Update IPC handlers
ipcMain.handle('check-for-update', async () => {
  return await checkForUpdate();
});

ipcMain.handle('open-download-page', async (_event, url: string) => {
  await shell.openExternal(url);
});

// Folder picker
ipcMain.handle('pick-folder', async () => {
  const result = await dialog.showOpenDialog(mainWindow!, {
    properties: ['openDirectory'],
  });
  if (result.canceled || result.filePaths.length === 0) return null;
  return result.filePaths[0];
});

// App lifecycle
app.whenReady().then(async () => {
  // Start pi-go backend before creating window
  try {
    await piGoManager.start();
  } catch (err) {
    console.error('Failed to start pi-go server:', err);
  }

  await createWindow();

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('before-quit', async () => {
  await piGoManager.stop();
});
