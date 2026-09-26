// easyagent-manager.ts — Manages the easyagent backend process lifecycle.
import { ChildProcess, spawn, execSync } from 'child_process';
import * as http from 'http';
import * as path from 'path';
import * as fs from 'fs';
import { app } from 'electron';

export interface EasyAgentServerInfo {
  url: string;
  port: number;
}

// Default .env content created on first run (for packaged app)
const DEFAULT_ENV_CONTENT = `# EasyAgent Configuration
# Edit this file to change the AI provider and model.

# Use OpenAI-compatible provider to connect to local gateway
EA_PROVIDER=openai
OPENAI_API_KEY=
OPENAI_BASE_URL=http://localhost:4001
OPENAI_MODEL=mimo-opus

# Enable bash tool
EA_ENABLE_BASH=true
`;

// Find an available port
function findFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = require('net').createServer();
    server.listen(0, '127.0.0.1', () => {
      const port = server.address().port;
      server.close(() => resolve(port));
    });
    server.on('error', reject);
  });
}

// Health check — poll until server is ready
function healthCheck(url: string, maxAttempts = 30, intervalMs = 500): Promise<void> {
  return new Promise((resolve, reject) => {
    let attempts = 0;
    const check = () => {
      attempts++;
      http
        .get(`${url}/health`, (res) => {
          if (res.statusCode === 200) {
            resolve();
          } else {
            retry();
          }
        })
        .on('error', () => {
          retry();
        });

      function retry() {
        if (attempts >= maxAttempts) {
          reject(new Error(`EasyAgent server not ready after ${maxAttempts} attempts`));
        } else {
          setTimeout(check, intervalMs);
        }
      }
    };
    check();
  });
}

export class EasyAgentManager {
  private process: ChildProcess | null = null;
  private serverInfo: EasyAgentServerInfo | null = null;

  async start(): Promise<EasyAgentServerInfo> {
    const port = await findFreePort();
    const url = `http://127.0.0.1:${port}`;
    const isPackaged = app.isPackaged;

    // Find easyagent binary
    const binary = this.findBinary(isPackaged);

    console.log(`[easyagent-manager] Starting ${binary} on port ${port} (packaged: ${isPackaged})`);

    // Prepare environment variables
    const envVars: Record<string, string> = {
      ...process.env as Record<string, string>,
    };

    let spawnCwd: string;

    if (isPackaged) {
      // --- Packaged mode ---
      const userDataPath = app.getPath('userData');
      const dataDir = path.join(userDataPath, 'data');

      // Ensure data directory exists
      fs.mkdirSync(dataDir, { recursive: true });

      // Setup .env in userData if not exists
      const envFile = path.join(userDataPath, '.env');
      if (!fs.existsSync(envFile)) {
        fs.writeFileSync(envFile, DEFAULT_ENV_CONTENT, 'utf-8');
        console.log(`[easyagent-manager] Created default .env at ${envFile}`);
      }

      // Tell Go process where to find data and config
      envVars.EA_DATA_DIR = dataDir;
      envVars.EA_ENV_FILE = envFile;

      // macOS: remove quarantine attributes from the binary
      if (process.platform === 'darwin') {
        try {
          execSync(`xattr -cr "${binary}"`, { stdio: 'ignore' });
        } catch (e) {
          console.warn('[easyagent-manager] Failed to remove quarantine attributes:', e);
        }
      }

      // Ensure binary is executable
      try {
        fs.chmodSync(binary, 0o755);
      } catch (e) {
        console.warn('[easyagent-manager] Failed to chmod binary:', e);
      }

      // cwd can be userDataPath for relative path resolution
      spawnCwd = userDataPath;
    } else {
      // --- Development mode ---
      // cwd is easyagent root so .env is loaded automatically
      spawnCwd = path.resolve(__dirname, '..', '..', '..');

      envVars.EA_PROVIDER = process.env.EA_PROVIDER || process.env.PI_GO_PROVIDER || 'openai';
      envVars.OPENAI_API_KEY = process.env.OPENAI_API_KEY || '';
      envVars.OPENAI_BASE_URL = process.env.OPENAI_BASE_URL || 'http://localhost:4001';
      envVars.OPENAI_MODEL = process.env.OPENAI_MODEL || 'mimo-opus';
    }

    this.process = spawn(binary, ['-mode', 'serve', '-listen', `127.0.0.1:${port}`], {
      cwd: spawnCwd,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: envVars,
    });

    this.process.stdout?.on('data', (data: Buffer) => {
      try {
        console.log(`[easyagent] ${data.toString().trim()}`);
      } catch (e) {
        // Ignore EPIPE when the process exits and the pipe breaks
      }
    });

    this.process.stderr?.on('data', (data: Buffer) => {
      try {
        console.error(`[easyagent] ${data.toString().trim()}`);
      } catch (e) {
        // Ignore EPIPE when the process exits and the pipe breaks
      }
    });

    this.process.on('exit', (code) => {
      console.log(`[easyagent-manager] Process exited with code ${code}`);
    });

    // Wait for server to be ready
    await healthCheck(url);

    this.serverInfo = { url, port };
    console.log(`[easyagent-manager] Server ready at ${url}`);

    return this.serverInfo;
  }

  async stop(): Promise<void> {
    if (this.process) {
      this.process.kill('SIGTERM');
      this.process = null;
      this.serverInfo = null;
    }
  }

  getServerInfo(): EasyAgentServerInfo | null {
    return this.serverInfo;
  }

  private findBinary(isPackaged: boolean): string {
    if (isPackaged) {
      // Packaged: binary is in Contents/Resources/easyagent
      const binaryPath = path.join(process.resourcesPath, 'easyagent');
      if (fs.existsSync(binaryPath)) {
        return binaryPath;
      }
      console.error(`[easyagent-manager] Binary not found at ${binaryPath}, falling back to PATH`);
      return 'easyagent';
    }

    // Development: try to find easyagent in the parent directory's build output
    const possiblePaths = [
      path.resolve(__dirname, '..', '..', '..', 'easyagent'),         // dist/electron → desktop → easyagent
      path.resolve(__dirname, '..', '..', '..', 'cmd', 'easyagent', 'easyagent'),
      'easyagent',  // Rely on PATH
    ];

    for (const p of possiblePaths) {
      if (fs.existsSync(p)) {
        return p;
      }
    }

    return 'easyagent';
  }
}
