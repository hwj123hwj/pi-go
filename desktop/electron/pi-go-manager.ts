// pi-go-manager.ts — Manages the pi-go backend process lifecycle.
import { ChildProcess, spawn, execSync } from 'child_process';
import * as http from 'http';
import * as path from 'path';
import * as fs from 'fs';
import { app } from 'electron';

export interface PiGoServerInfo {
  url: string;
  port: number;
}

// Default .env content created on first run (for packaged app)
const DEFAULT_ENV_CONTENT = `# Pi-Go Configuration
# Edit this file to change the AI provider and model.

PI_GO_PROVIDER=deepv
DEEPV_ENABLED=true
DEEPV_MODEL=deepseek-v4-flash
# DEEPV_WORK_DIR=/path/to/your/project

# OpenAI-compatible (alternative)
# PI_GO_PROVIDER=openai
# OPENAI_API_KEY=your-api-key
# OPENAI_MODEL=gpt-4
# OPENAI_BASE_URL=https://api.openai.com/v1

# Enable bash tool
PI_GO_ENABLE_BASH=true
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
          reject(new Error(`pi-go server not ready after ${maxAttempts} attempts`));
        } else {
          setTimeout(check, intervalMs);
        }
      }
    };
    check();
  });
}

export class PiGoManager {
  private process: ChildProcess | null = null;
  private serverInfo: PiGoServerInfo | null = null;

  async start(): Promise<PiGoServerInfo> {
    const port = await findFreePort();
    const url = `http://127.0.0.1:${port}`;
    const isPackaged = app.isPackaged;

    // Find pi-agent binary
    const binary = this.findBinary(isPackaged);

    console.log(`[pi-go-manager] Starting ${binary} on port ${port} (packaged: ${isPackaged})`);

    // Prepare environment variables
    const envVars: Record<string, string> = {
      ...process.env as Record<string, string>,
      PI_GO_ENABLE_BASH: process.env.PI_GO_ENABLE_BASH || 'true',
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
        console.log(`[pi-go-manager] Created default .env at ${envFile}`);
      }

      // Tell Go process where to find data and config
      envVars.PI_GO_DATA_DIR = dataDir;
      envVars.PI_GO_ENV_FILE = envFile;

      // macOS: remove quarantine attributes from the binary
      if (process.platform === 'darwin') {
        try {
          execSync(`xattr -cr "${binary}"`, { stdio: 'ignore' });
        } catch (e) {
          console.warn('[pi-go-manager] Failed to remove quarantine attributes:', e);
        }
      }

      // Ensure binary is executable
      try {
        fs.chmodSync(binary, 0o755);
      } catch (e) {
        console.warn('[pi-go-manager] Failed to chmod binary:', e);
      }

      // cwd can be userDataPath for relative path resolution
      spawnCwd = userDataPath;
    } else {
      // --- Development mode ---
      // cwd is pi-go root so .env is loaded automatically
      spawnCwd = path.resolve(__dirname, '..', '..', '..');

      envVars.PI_GO_PROVIDER = process.env.PI_GO_PROVIDER || 'deepv';
      envVars.DEEPV_ENABLED = process.env.DEEPV_ENABLED || 'true';
      envVars.DEEPV_MODEL = process.env.DEEPV_MODEL || 'deepseek-v4-flash';
    }

    this.process = spawn(binary, ['-mode', 'serve', '-listen', `127.0.0.1:${port}`], {
      cwd: spawnCwd,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: envVars,
    });

    this.process.stdout?.on('data', (data: Buffer) => {
      console.log(`[pi-go] ${data.toString().trim()}`);
    });

    this.process.stderr?.on('data', (data: Buffer) => {
      console.error(`[pi-go] ${data.toString().trim()}`);
    });

    this.process.on('exit', (code) => {
      console.log(`[pi-go-manager] Process exited with code ${code}`);
    });

    // Wait for server to be ready
    await healthCheck(url);

    this.serverInfo = { url, port };
    console.log(`[pi-go-manager] Server ready at ${url}`);

    return this.serverInfo;
  }

  async stop(): Promise<void> {
    if (this.process) {
      this.process.kill('SIGTERM');
      this.process = null;
      this.serverInfo = null;
    }
  }

  getServerInfo(): PiGoServerInfo | null {
    return this.serverInfo;
  }

  private findBinary(isPackaged: boolean): string {
    if (isPackaged) {
      // Packaged: binary is in Contents/Resources/pi-agent
      const binaryPath = path.join(process.resourcesPath, 'pi-agent');
      if (fs.existsSync(binaryPath)) {
        return binaryPath;
      }
      console.error(`[pi-go-manager] Binary not found at ${binaryPath}, falling back to PATH`);
      return 'pi-agent';
    }

    // Development: try to find pi-agent in the parent directory's build output
    const possiblePaths = [
      path.resolve(__dirname, '..', '..', '..', 'pi-agent'),         // dist/electron → desktop → pi-go
      path.resolve(__dirname, '..', '..', '..', 'cmd', 'pi-agent', 'pi-agent'),
      'pi-agent',  // Rely on PATH
    ];

    for (const p of possiblePaths) {
      if (fs.existsSync(p)) {
        return p;
      }
    }

    return 'pi-agent';
  }
}
