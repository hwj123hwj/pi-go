// main.ts — Electron main process for pi-go desktop client.
import { app, BrowserWindow, ipcMain, shell, dialog } from 'electron';
import * as path from 'path';
import { spawn } from 'child_process';
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

// Reveal a file in the OS file manager (Finder on macOS, Explorer on Windows).
ipcMain.handle('reveal-in-folder', async (_event, filePath: string) => {
  try {
    if (process.platform === 'darwin') {
      // On macOS, open the parent folder with the file selected.
      shell.showItemInFolder(filePath);
    } else {
      // On Windows/Linux, open the containing directory.
      const dir = path.dirname(filePath);
      await shell.openPath(dir);
    }
  } catch {
    // best-effort
  }
});

// Open a terminal at the given directory.
ipcMain.handle('open-in-terminal', async (_event, dir: string) => {
  try {
    if (process.platform === 'darwin') {
      // macOS: open Terminal.app at the directory
      spawn('open', ['-a', 'Terminal', dir]);
    } else if (process.platform === 'win32') {
      // Windows: open cmd/PowerShell
      spawn('cmd', ['/c', 'start', 'cmd', '/k', `cd /d ${dir}`], { shell: true });
    } else {
      // Linux: try common terminal emulators
      const terminals = ['gnome-terminal', 'konsole', 'xfce4-terminal', 'xterm'];
      for (const term of terminals) {
        try {
          spawn(term, ['--working-directory', dir]);
          return;
        } catch {
          // try next
        }
      }
    }
  } catch {
    // best-effort
  }
});

// Open a URL in the system default browser.
ipcMain.handle('open-external', async (_event, url: string) => {
  try {
    await shell.openExternal(url);
  } catch {
    // best-effort
  }
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
