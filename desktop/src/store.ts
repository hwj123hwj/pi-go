/**
 * store.ts — Single source of truth for the pi-go desktop renderer.
 *
 * Talks directly to the pi-go backend via REST (fetch) + WebSocket.
 * No IPC bridge — the renderer hits the HTTP server managed by Electron's
 * pi-go-manager directly.
 */

import { create } from 'zustand';
import { type Lang, loadStoredLang, persistLang, translate } from './i18n/i18n';
import { type ThemeMode, loadStoredTheme, persistTheme } from './theme';
import { deriveTitleFromMessage } from './sessionTitle';
import type {
  AcpToolKind,
  DesktopSessionEvent,
  GitFileDiff,
  ModelInfo,
  PlanEntry,
  SessionMeta,
  SessionRunStatus,
  ToolCallContent,
  ToolCallStatus,
  ToolLocation,
  UpdateInfo,
  UpdateState,
} from './types';

// ── REST API helpers ──────────────────────────────────────────────────────

let baseUrl = 'http://127.0.0.1:8080';

export function setBaseUrl(url: string): void {
  baseUrl = url;
}

export function getBaseUrl(): string {
  return baseUrl;
}

// Extract file paths from tool result text for clickable locations.
// Matches: paths with known extensions, paths after "文件:" prefix,
// directory paths (ending with /), and paths with ≥2 segments but no extension.
function extractLocationsFromText(text: string): ToolLocation[] {
  const locations: ToolLocation[] = [];
  const seen = new Set<string>();

  const knownExt = '(?:md|txt|json|js|ts|go|py|yaml|yml|toml|xml|html|css|sh|bash|rs|java|c|cpp|h|rb|php|sql|graphql|proto|vue|svelte|jsx|tsx|mdx|csv|log|cfg|conf|ini|env|lock|sum|mod)';

  // Pattern 1: paths explicitly labeled (文件: /path/to/file)
  // This catches both files and directories after a label prefix.
  const pathPatterns = [
    /(?:文件|File|路径|Path)[:：]\s*(\/[^\s\n]+)/gi,
    // Paths with known extensions (broad match)
    new RegExp(`(\\/[^\\s\\n]+\\.${knownExt})`, 'gi'),
    // Directory paths ending with / (e.g. /Users/weijian/agent-lessons/doubao-knowledge/work/)
    /(\/(?:[^\s\n]+\/){2,})/g,
    // Paths with ≥3 segments, no extension in last segment
    // (e.g. /Users/weijian/agent-lessons/doubao-knowledge/other)
    // Requires ≥3 segments to avoid matching short fragments like "/foo/bar"
    /(\/[^\s\n]*\/[^\s\n]*\/[^\s\n/.]+(?:\/[^\s\n/.]+)*)/g,
  ];

  for (const pattern of pathPatterns) {
    let match;
    while ((match = pattern.exec(text)) !== null) {
      let path = match[1].trim();
      // Skip URLs
      if (/^https?:\/\//i.test(path)) continue;
      // Skip very short matches (avoid false positives like "/a")
      if (path.length < 10) continue;
      // Normalize: remove trailing / for dedup, but keep it for display
      const normalized = path.replace(/\/+$/, '');
      if (!seen.has(normalized)) {
        seen.add(normalized);
        locations.push({ path });
      }
    }
  }

  // Pattern 2: paths without extensions that have ≥2 segments
  // (e.g. /Users/weijian/agent-lessons/doubao-knowledge/work/something)
  // Only match if it follows a label prefix to avoid false positives.
  const noExtPattern = /(?:文件|File|路径|Path)[:：]\s*(\/(?:[^\s\n]+\/)*[^\s\n/.]+)/gi;
  let match;
  while ((match = noExtPattern.exec(text)) !== null) {
    const path = match[1].trim();
    if (path.length < 10) continue;
    const normalized = path.replace(/\/+$/, '');
    if (!seen.has(normalized)) {
      seen.add(normalized);
      locations.push({ path });
    }
  }

  return locations;
}

async function apiRequest<T>(method: string, path: string, body?: unknown): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
  };
  if (body !== undefined) {
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(`${baseUrl}${path}`, opts);
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `HTTP ${res.status}`);
  }
  return res.json();
}

// ── WebSocket ─────────────────────────────────────────────────────────────

type MessageHandler = (data: any) => void;

class WSService {
  private ws: WebSocket | null = null;
  private handlers: Map<string, MessageHandler[]> = new Map();
  private _connected = false;
  private reconnectAttempts = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  get connected(): boolean {
    return this._connected;
  }

  connect(url: string): void {
    if (this._connected && this.ws && this.ws.readyState === WebSocket.OPEN) return;
    this.disconnect();
    const wsUrl = url.replace(/^http/, 'ws') + '/ws';
    console.log(`[ws] Connecting to ${wsUrl}`);
    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      console.log('[ws] Connected');
      this._connected = true;
      this.reconnectAttempts = 0; // reset backoff on successful connection
      this.emit('connected', {});
    };

    this.ws.onclose = () => {
      console.log('[ws] Disconnected');
      this._connected = false;
      this.emit('disconnected', {});
      // Auto-reconnect (exponential backoff: 2s → 4s → 8s, max 30s)
      const delay = Math.min(2000 * Math.pow(2, this.reconnectAttempts), 30000);
      this.reconnectAttempts++;
      this.reconnectTimer = setTimeout(() => this.connect(url), delay);
    };

    this.ws.onerror = () => {
      this.emit('error', { error: 'WebSocket error' });
    };

    this.ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        this.routeMessage(data);
      } catch (err) {
        console.error('[ws] Failed to parse message', err);
      }
    };
  }

  private routeMessage(data: any): void {
    this.emit('message', data);
    const type = data.type;
    if (type === 'event' && data.event) {
      this.emit(`event:${data.event.type}`, data);
    } else {
      this.emit(`type:${type}`, data);
    }
  }

  send(data: object): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    } else {
      console.warn('[ws] Cannot send, not connected');
    }
  }

  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.close();
      this.ws = null;
    }
    this._connected = false;
  }

  on(event: string, handler: MessageHandler): () => void {
    if (!this.handlers.has(event)) this.handlers.set(event, []);
    this.handlers.get(event)!.push(handler);
    return () => {
      const handlers = this.handlers.get(event);
      if (handlers) {
        const idx = handlers.indexOf(handler);
        if (idx >= 0) handlers.splice(idx, 1);
      }
    };
  }

  private emit(event: string, data: any): void {
    const handlers = this.handlers.get(event);
    if (handlers) handlers.forEach((h) => h(data));
  }
}

export const wsService = new WSService();

// ── View models ────────────────────────────────────────────────────────────

export type ViewDensity = 'normal' | 'verbose' | 'summary';

export type PaneKind = 'chat' | 'diff' | 'plan' | 'tasks' | 'terminal' | 'file';

/** Which feature the right workspace sidebar is showing. */
export type RightView = 'review' | 'files' | 'plan' | 'tasks' | 'kb' | 'profile';

/**
 * Global workspace layout — the right feature sidebar + the bottom terminal.
 * App-level (NOT per-session) so the chosen layout survives session switches.
 * Persisted to localStorage.
 */
export interface WorkspaceUiState {
  /** Whether the left session-list sidebar is expanded (vs. collapsed). */
  sidebarOpen: boolean;
  /** Width (px) of the left session-list sidebar. */
  sidebarWidth: number;
  rightOpen: boolean;
  /**
   * Which feature panel is open, or null for "launcher" mode: the right rail
   * is shown with full labels and no content panel.
   */
  rightView: RightView | null;
  bottomOpen: boolean;
  rightWidth: number;
  bottomHeight: number;
  fileTreeWidth: number;
  /** Absolute paths of files open in the Files panel (VSCode-style tabs). */
  fileTabs: string[];
  activeFileTab?: string;
}

/** Clamp ranges for the draggable regions. */
export const WORKSPACE_SIZE_LIMITS = {
  sidebarWidth: { min: 200, max: 480, default: 260 },
  rightWidth: { min: 340, max: 900, default: 520 },
  bottomHeight: { min: 120, max: 720, default: 280 },
  fileTreeWidth: { min: 160, max: 480, default: 220 },
} as const;

export type ChatItem =
  | { kind: 'user'; id: string; text: string }
  | { kind: 'assistant'; id: string; text: string }
  | { kind: 'thought'; id: string; text: string }
  | { kind: 'system'; id: string; text: string }
  | { kind: 'error'; id: string; text: string }
  | {
      kind: 'tool';
      id: string;
      toolCallId: string;
      title: string;
      toolKind: AcpToolKind;
      status: ToolCallStatus;
      locations?: ToolLocation[];
      content: ToolCallContent[];
      terminalOutput?: string;
      rawInput?: Record<string, unknown>;
      details?: Record<string, unknown>; // 结构化结果（如 music_play 的 PlayDetails），前端据此渲染
    };

export interface SessionView {
  meta: SessionMeta;
  transcript: ChatItem[];
  plan: PlanEntry[];
  diffs: GitFileDiff[];
  density: ViewDensity;
  panes: PaneKind[];
  activePane: PaneKind;
  draftAssistantId?: string;
  openFile?: { path: string; content: string };
}

interface StoreState {
  ready: boolean;
  connected: boolean;
  sessions: Record<string, SessionView>;
  order: string[];
  activeSessionId?: string;
  lang: Lang;
  setLang: (lang: Lang) => void;
  theme: ThemeMode;
  setTheme: (theme: ThemeMode) => void;

  // Models fetched dynamically from backend
  models: ModelInfo[];
  currentModel?: string;
  pickFolder: () => Promise<string | null>;

  update: UpdateState | null;
  checkUpdate: () => Promise<void>;
  downloadUpdate: () => Promise<void>;
  snoozeUpdate: () => void;

  // ── Workspace layout ──
  workspace: WorkspaceUiState;
  toggleSidebar: () => void;
  toggleWorkspaceRight: () => void;
  toggleWorkspaceBottom: () => void;
  openWorkspaceView: (view: RightView) => void;
  setWorkspaceSize: (key: keyof typeof WORKSPACE_SIZE_LIMITS, value: number) => void;
  openFileTab: (path: string) => void;
  closeFileTab: (path: string) => void;
  setActiveFileTab: (path: string) => void;

  init: () => Promise<void>;
  refreshSessions: () => Promise<void>;
  setActive: (id: string) => Promise<void>;
  createSession: (opts?: { cwd?: string; model?: string; application?: string }) => Promise<string>;
  deleteSession: (id: string) => Promise<void>;
  sendPrompt: (id: string, text: string) => Promise<void>;
  cancel: (id: string) => Promise<void>;
  setModel: (id: string, modelId: string) => Promise<void>;
  setDensity: (id: string, density: ViewDensity) => void;
  togglePane: (id: string, pane: PaneKind) => void;
  refreshDiff: (id: string) => Promise<void>;
  openFile: (id: string, path: string) => Promise<void>;
  saveFile: (id: string, path: string, content: string) => Promise<boolean>;

  // ── Global music player ──
  music: MusicState;
  playMusic: (song: MusicTrack) => void;
  setMusicPlaying: (playing: boolean) => void;
  setMusicTime: (time: number) => void;
  setMusicDuration: (duration: number) => void;
  setMusicError: (error: boolean) => void;
  toggleMusic: () => void;
  clearMusic: () => void;
}

export interface MusicTrack {
  songName: string;
  artist: string;
  audioURL: string;
  duration?: number; // seconds
  sessionId?: string;
}

export interface MusicState {
  current: MusicTrack | null;
  playing: boolean;
  currentTime: number;
  duration: number;
  error: boolean;
}

// Cached models from backend (shared across all sessions)
let cachedModels: ModelInfo[] = [];
let cachedCurrentModel: string | undefined;

let initialized = false;

const newId = (() => {
  let n = 0;
  return () => `r${Date.now().toString(36)}-${(n++).toString(36)}`;
})();

function emptyView(meta: SessionMeta): SessionView {
  return {
    meta,
    transcript: [],
    plan: [],
    diffs: [],
    density: 'normal',
    panes: ['chat'],
    activePane: 'chat',
  };
}

function defaultModels(): ModelInfo[] {
  if (cachedModels.length > 0) return cachedModels;
  return [
    { modelId: 'deepseek-v4-flash', name: 'DeepSeek V4 Flash' },
    { modelId: 'glm-5', name: 'GLM-5' },
    { modelId: 'claude-sonnet-4-6', name: 'Claude Sonnet 4.6' },
  ];
}

async function fetchModels(): Promise<void> {
  try {
    const resp = await apiRequest<{ models: Array<{ id: string; name: string; provider: string }>; current?: { id: string } }>('GET', '/models');
    if (resp.models && resp.models.length > 0) {
      cachedModels = resp.models.map((m) => ({ modelId: m.id, name: m.name }));
      cachedCurrentModel = resp.current?.id;
    }
  } catch (err) {
    console.error('Failed to fetch models', err);
  }
}

// ── Workspace layout persistence ────────────────────────────────────────────

const WORKSPACE_KEY = 'pi-go.workspace';

function clampSize(v: unknown, key: keyof typeof WORKSPACE_SIZE_LIMITS): number {
  const { min, max, default: dflt } = WORKSPACE_SIZE_LIMITS[key];
  if (typeof v !== 'number' || !Number.isFinite(v)) return dflt;
  return Math.min(max, Math.max(min, v));
}

function loadWorkspaceUi(): WorkspaceUiState {
  const base: WorkspaceUiState = {
    sidebarOpen: true,
    sidebarWidth: WORKSPACE_SIZE_LIMITS.sidebarWidth.default,
    rightOpen: false,
    rightView: null,
    bottomOpen: false,
    rightWidth: WORKSPACE_SIZE_LIMITS.rightWidth.default,
    bottomHeight: WORKSPACE_SIZE_LIMITS.bottomHeight.default,
    fileTreeWidth: WORKSPACE_SIZE_LIMITS.fileTreeWidth.default,
    fileTabs: [],
  };
  try {
    const raw = localStorage.getItem(WORKSPACE_KEY);
    if (raw) {
      const p = JSON.parse(raw) as Partial<WorkspaceUiState>;
      return {
        ...base,
        sidebarOpen: p.sidebarOpen ?? true,
        sidebarWidth: clampSize(p.sidebarWidth, 'sidebarWidth'),
        rightOpen: !!p.rightOpen,
        rightView: p.rightView ?? null,
        bottomOpen: !!p.bottomOpen,
        rightWidth: clampSize(p.rightWidth, 'rightWidth'),
        bottomHeight: clampSize(p.bottomHeight, 'bottomHeight'),
        fileTreeWidth: clampSize(p.fileTreeWidth, 'fileTreeWidth'),
      };
    }
  } catch {
    /* localStorage unavailable / malformed */
  }
  return base;
}

function persistWorkspaceUi(w: WorkspaceUiState): void {
  try {
    localStorage.setItem(
      WORKSPACE_KEY,
      JSON.stringify({
        sidebarOpen: w.sidebarOpen,
        sidebarWidth: w.sidebarWidth,
        rightOpen: w.rightOpen,
        rightView: w.rightView,
        bottomOpen: w.bottomOpen,
        rightWidth: w.rightWidth,
        bottomHeight: w.bottomHeight,
        fileTreeWidth: w.fileTreeWidth,
      }),
    );
  } catch {
    /* best-effort */
  }
}

export const useStore = create<StoreState>((set, get) => ({
  ready: false,
  connected: false,
  sessions: {},
  order: [],
  models: [],
  currentModel: undefined,
  workspace: loadWorkspaceUi(),
  pickFolder: async () => {
    return await window.piAPI?.pickFolder() ?? null;
  },
  lang: loadStoredLang(),
  setLang: (lang) => {
    persistLang(lang);
    set({ lang });
  },
  theme: loadStoredTheme(),
  setTheme: (theme) => {
    persistTheme(theme);
    set({ theme });
  },

  // ── Global music player ──
  music: { current: null, playing: false, currentTime: 0, duration: 0, error: false },
  playMusic: (song) => {
    set((s) => ({
      music: { ...s.music, current: song, playing: true, currentTime: 0, duration: song.duration || 0, error: false },
    }));
  },
  setMusicPlaying: (playing) => set((s) => ({ music: { ...s.music, playing } })),
  setMusicTime: (time) => set((s) => ({ music: { ...s.music, currentTime: time } })),
  setMusicDuration: (duration) => set((s) => ({ music: { ...s.music, duration } })),
  setMusicError: (error) => set((s) => ({ music: { ...s.music, error } })),
  toggleMusic: () => set((s) => ({ music: { ...s.music, playing: !s.music.playing } })),
  clearMusic: () => set((s) => ({ music: { current: null, playing: false, currentTime: 0, duration: 0, error: false } })),

  update: null,
  checkUpdate: async () => {
    try {
      const info = await window.piAPI?.checkForUpdate();
      if (info) {
        set({ update: { supported: true, phase: 'available', info, currentVersion: __APP_VERSION__ } });
      } else {
        set({ update: { supported: true, phase: 'idle', info: null, currentVersion: __APP_VERSION__ } });
      }
    } catch {
      set({ update: { supported: true, phase: 'error', error: 'Check failed', currentVersion: __APP_VERSION__ } });
    }
  },
  downloadUpdate: async () => {
    const u = get().update;
    if (u?.info?.downloadUrl) {
      await window.piAPI?.openDownloadPage(u.info.downloadUrl);
    }
    set((s) => ({ update: s.update ? { ...s.update, phase: 'idle' } : null }));
  },
  snoozeUpdate: () =>
    set((s) => (s.update ? { update: { ...s.update, snoozed: true } } : {})),

  // ── Workspace layout actions ──
  toggleSidebar: () => {
    set((s) => {
      const workspace = { ...s.workspace, sidebarOpen: !s.workspace.sidebarOpen };
      persistWorkspaceUi(workspace);
      return { workspace };
    });
  },
  toggleWorkspaceRight: () => {
    set((s) => {
      // When opening, default to 'files' if no view is selected yet.
      const rightOpen = !s.workspace.rightOpen;
      const rightView = rightOpen ? (s.workspace.rightView ?? 'files') : s.workspace.rightView;
      const workspace: WorkspaceUiState = { ...s.workspace, rightOpen, rightView };
      persistWorkspaceUi(workspace);
      return { workspace };
    });
  },
  toggleWorkspaceBottom: () => {
    set((s) => {
      const workspace = { ...s.workspace, bottomOpen: !s.workspace.bottomOpen };
      persistWorkspaceUi(workspace);
      return { workspace };
    });
  },
  openWorkspaceView: (view) => {
    set((s) => {
      const workspace = { ...s.workspace, rightOpen: true, rightView: view };
      persistWorkspaceUi(workspace);
      return { workspace };
    });
  },
  setWorkspaceSize: (key, value) => {
    set((s) => {
      const clamped = clampSize(value, key);
      const workspace = { ...s.workspace, [key]: clamped };
      persistWorkspaceUi(workspace);
      return { workspace };
    });
  },
  openFileTab: (path) => {
    set((s) => {
      const tabs = s.workspace.fileTabs.includes(path)
        ? s.workspace.fileTabs
        : [...s.workspace.fileTabs, path];
      const workspace: WorkspaceUiState = { ...s.workspace, fileTabs: tabs, activeFileTab: path, rightOpen: true, rightView: 'files' as RightView };
      persistWorkspaceUi(workspace);
      return { workspace };
    });
  },
  closeFileTab: (path) => {
    set((s) => {
      const tabs = s.workspace.fileTabs.filter((t) => t !== path);
      const activeFileTab = s.workspace.activeFileTab === path
        ? (tabs[tabs.length - 1] ?? undefined)
        : s.workspace.activeFileTab;
      const workspace: WorkspaceUiState = { ...s.workspace, fileTabs: tabs, activeFileTab };
      persistWorkspaceUi(workspace);
      return { workspace };
    });
  },
  setActiveFileTab: (path) => {
    set((s) => {
      const workspace = { ...s.workspace, activeFileTab: path };
      persistWorkspaceUi(workspace);
      return { workspace };
    });
  },

  init: async () => {
    if (initialized) return;
    initialized = true;

    // Get server URL from Electron main process
    const serverUrl = await window.piAPI?.getServerUrl();
    if (serverUrl) {
      setBaseUrl(serverUrl);
    }

    // Connect WebSocket
    const wsUrl = getBaseUrl();
    wsService.connect(wsUrl);

    wsService.on('connected', () => set({ connected: true }));
    wsService.on('disconnected', () => set({ connected: false }));

    // ── WebSocket event handlers ──

    wsService.on('type:session_id', (data: any) => {
      // Server may send a session_id response
    });

    wsService.on('type:status', (data: any) => {
      const sessionId = data.session_id;
      if (!sessionId) return;
      if (!data.streaming) {
        // Streaming done — finalize the assistant message
        updateView(set, sessionId, (v) => {
          const transcript = [...v.transcript];
          const last = transcript[transcript.length - 1];
          if (last && last.kind === 'assistant') {
            transcript[transcript.length - 1] = { ...last };
          }
          return {
            ...v,
            transcript,
            meta: { ...v.meta, status: 'idle' as SessionRunStatus },
            draftAssistantId: undefined,
          };
        });
      }
    });

    wsService.on('event:text_delta', (data: any) => {
      const sessionId = data.session_id;
      const delta = data.event?.text_delta || '';
      if (!sessionId || !delta) return;
      updateView(set, sessionId, (v) => {
        const transcript = [...v.transcript];
        const last = transcript[transcript.length - 1];
        if (last && last.kind === 'assistant') {
          // Append to existing assistant message
          transcript[transcript.length - 1] = { ...last, text: last.text + delta };
        } else {
          // Last item is a tool/system/etc — create a new assistant item
          const id = v.draftAssistantId || newId();
          transcript.push({ kind: 'assistant', id, text: delta });
          return { ...v, transcript, draftAssistantId: id, meta: { ...v.meta, status: 'thinking' as SessionRunStatus } };
        }
        return { ...v, transcript, meta: { ...v.meta, status: 'thinking' as SessionRunStatus } };
      });
    });

    wsService.on('event:tool_start', (data: any) => {
      const sessionId = data.session_id;
      if (!sessionId) return;
      const toolName = data.event?.tool_name || 'tool';
      const toolCallId = data.event?.tool_call_id || newId();
      const toolKind = inferToolKind(toolName);
      const item: ChatItem = {
        kind: 'tool',
        id: newId(),
        toolCallId,
        title: toolName,
        toolKind,
        status: 'in_progress',
        content: [],
      };
      updateView(set, sessionId, (v) => ({
        ...v,
        transcript: [...v.transcript, item],
      }));
    });

    wsService.on('event:tool_end', (data: any) => {
      const sessionId = data.session_id;
      if (!sessionId) return;
      const toolCallId = data.event?.tool_call_id || '';
      const result = data.event?.tool_result;
      const isError = data.event?.is_error || false;
      // result 可能是字符串（老格式）或 ToolResult 对象（{Content, UserFacing, Details}）
      // 展示文本优先 UserFacing，次 Content，兼容字符串
      const resultObj = result && typeof result === 'object' ? result : null;
      const resultText =
        (resultObj && (resultObj.UserFacing || resultObj.Content)) ||
        (typeof result === 'string' ? result : JSON.stringify(result || ''));
      const details = resultObj?.Details ?? data.event?.tool_details ?? undefined;

      // Extract file paths from result text for clickable locations
      const locations = extractLocationsFromText(resultText);

      updateView(set, sessionId, (v) => {
        const transcript = v.transcript.map((item) => {
          if (item.kind === 'tool' && item.toolCallId === toolCallId) {
            return {
              ...item,
              status: (isError ? 'failed' : 'completed') as ToolCallStatus,
              content: [{ text: resultText }],
              details,
              locations: locations.length > 0 ? locations : item.locations,
            };
          }
          return item;
        });
        return { ...v, transcript };
      });
    });

    wsService.on('event:turn_end', () => {
      // Turn ended — backend status message will finalize
    });

    wsService.on('event:error', (data: any) => {
      const sessionId = data.session_id;
      const error = data.event?.error || 'Unknown error';
      if (!sessionId) return;
      const item: ChatItem = { kind: 'error', id: newId(), text: error };
      updateView(set, sessionId, (v) => ({
        ...v,
        transcript: [...v.transcript, item],
        meta: { ...v.meta, status: 'error' as SessionRunStatus },
      }));
    });

    wsService.on('type:error', (data: any) => {
      const sessionId = data.session_id;
      if (sessionId) {
        const item: ChatItem = { kind: 'error', id: newId(), text: data.message || 'Unknown error' };
        updateView(set, sessionId, (v) => ({
          ...v,
          transcript: [...v.transcript, item],
          meta: { ...v.meta, status: 'error' as SessionRunStatus },
        }));
      }
    });

    // Fetch models from backend (dynamic from gateway)
    await fetchModels();
    set({ models: cachedModels, currentModel: cachedCurrentModel });

    // Load existing sessions
    await get().refreshSessions();
    set({ ready: true });
  },

  refreshSessions: async () => {
    try {
      const raw = await apiRequest<any[]>('GET', '/sessions');
      const sessions: any[] = Array.isArray(raw) ? raw : [];
      set((s) => {
        const newSessions: Record<string, SessionView> = {};
        for (const sess of sessions) {
          // Preserve existing cwd if the backend doesn't provide one
          const existingCwd = s.sessions[sess.id]?.meta.cwd;
          const cwd = sess.workspace || existingCwd || '';
          // Derive title: prefer backend title, then existing local title, then fallback
          const backendTitle = sess.title ? deriveTitleFromMessage(sess.title) : undefined;
          const existingTitle = s.sessions[sess.id]?.meta.title;
          // If existing title is the auto-generated fallback, prefer backend title
          const isFallback = !existingTitle || existingTitle.startsWith('Session ');
          const title = isFallback ? (backendTitle || existingTitle || `Session ${sess.id.slice(-6)}`) : existingTitle;
          // Read application from backend, fallback to existing
          const application = sess.application || s.sessions[sess.id]?.meta.application;
          const meta: SessionMeta = {
            id: sess.id,
            title,
            cwd,
            status: 'idle' as SessionRunStatus,
            model: s.sessions[sess.id]?.meta.model,
            application,
            availableModels: defaultModels(),
            createdAt: sess.created_at || 0,
            updatedAt: sess.last_active || 0,
          };
          newSessions[sess.id] = s.sessions[sess.id] ?? emptyView(meta);
          // Update meta for existing sessions (in case cwd/title/application was loaded from backend)
          if (s.sessions[sess.id]) {
            newSessions[sess.id] = {
              ...s.sessions[sess.id],
              meta: {
                ...s.sessions[sess.id].meta,
                cwd,
                title,
                application,
                updatedAt: sess.last_active || 0,
              },
            };
          }
        }
        return { sessions: newSessions, order: sessions.map((s) => s.id) };
      });
    } catch (err) {
      console.error('Failed to load sessions', err);
    }
  },

  setActive: async (id) => {
    set({ activeSessionId: id });
    // Load transcript from backend if this session has no messages yet
    const view = get().sessions[id];
    if (view && view.transcript.length === 0) {
      try {
        const messages = await apiRequest<any[]>('GET', `/sessions/${id}/messages`);
        if (!Array.isArray(messages) || messages.length === 0) return;

        const items: ChatItem[] = [];
        for (const msg of messages) {
          if (msg.role === 'user' && msg.content) {
            items.push({ kind: 'user', id: newId(), text: msg.content });
          } else if (msg.role === 'assistant') {
            // Thinking
            if (msg.thinking) {
              items.push({ kind: 'thought', id: newId(), text: msg.thinking });
            }
            // Tool calls (each becomes a completed tool item)
            if (msg.tool_calls && msg.tool_calls.length > 0) {
              for (const tc of msg.tool_calls) {
                const toolKind = inferToolKind(tc.name);
                // Find matching tool result
                const resultMsg = messages.find(
                  (m: any) => m.role === 'tool' && m.tool_call_id === tc.id,
                );
                const resultText = resultMsg?.content || '';
                const isError = resultMsg?.is_error || false;
                const toolDetails = resultMsg?.tool_details ?? undefined;
                items.push({
                  kind: 'tool',
                  id: newId(),
                  toolCallId: tc.id,
                  title: tc.name,
                  toolKind,
                  status: isError ? 'failed' : 'completed',
                  content: [{ text: resultText }],
                  details: toolDetails,
                  rawInput: tc.args ? (() => { try { return JSON.parse(tc.args); } catch { return undefined; } })() : undefined,
                });
              }
            }
            // Text response
            if (msg.content) {
              items.push({ kind: 'assistant', id: newId(), text: msg.content });
            }
          }
        }

        // Derive title from first user message
        const firstUser = messages.find((m: any) => m.role === 'user');
        const title = firstUser?.content ? deriveTitleFromMessage(firstUser.content) : undefined;

        if (items.length > 0) {
          updateView(set, id, (v) => ({
            ...v,
            transcript: items,
            ...(title ? { meta: { ...v.meta, title } } : {}),
          }));
        }
      } catch (err) {
        console.error('Failed to load session transcript', err);
      }
    }
  },

  createSession: async (opts) => {
    const body: Record<string, string> = {};
    if (opts?.cwd) body.cwd = opts.cwd;
    if (opts?.model) body.model = opts.model;
    if (opts?.application) body.application = opts.application;
    const result = await apiRequest<{ id: string; created_at: number }>('POST', '/sessions', body);
    const lang = get().lang;
    const meta: SessionMeta = {
      id: result.id,
      title: opts?.application === 'music'
        ? translate(lang, 'session.musicTitle')
        : opts?.cwd ? projectName(opts.cwd) : translate(lang, 'session.defaultTitle'),
      cwd: opts?.cwd || '',
      status: 'idle' as SessionRunStatus,
      model: opts?.model || cachedCurrentModel,
      application: opts?.application,
      availableModels: defaultModels(),
      createdAt: result.created_at || Date.now(),
      updatedAt: Date.now(),
    };
    set((s) => ({
      sessions: { ...s.sessions, [result.id]: emptyView(meta) },
      order: [result.id, ...s.order],
      activeSessionId: result.id,
    }));
    return result.id;
  },

  deleteSession: async (id) => {
    await apiRequest('DELETE', `/sessions/${id}`);
    set((s) => {
      const sessions = { ...s.sessions };
      delete sessions[id];
      const order = s.order.filter((x) => x !== id);
      const activeSessionId = s.activeSessionId === id ? (order[0] ?? undefined) : s.activeSessionId;
      return { sessions, order, activeSessionId };
    });
  },

  sendPrompt: async (id, text) => {
    // Optimistic user message
    const isFirst = !get().sessions[id]?.transcript.some((i) => i.kind === 'user');
    if (isFirst) {
      const title = deriveTitleFromMessage(text);
      if (title) {
        updateView(set, id, (v) => ({ ...v, meta: { ...v.meta, title } }));
      }
    }

    const userItem: ChatItem = { kind: 'user', id: newId(), text };
    const assistantItem: ChatItem = { kind: 'assistant', id: newId(), text: '' };
    updateView(set, id, (v) => ({
      ...v,
      transcript: [...v.transcript, userItem, assistantItem],
      meta: { ...v.meta, status: 'thinking' as SessionRunStatus },
      draftAssistantId: assistantItem.id,
    }));

    // Send via WebSocket
    wsService.send({ type: 'prompt', session_id: id, prompt: text });
  },

  cancel: async (id) => {
    wsService.send({ type: 'cancel', session_id: id });
    updateView(set, id, (v) => ({ ...v, meta: { ...v.meta, status: 'idle' as SessionRunStatus } }));
  },

  setModel: async (id, modelId) => {
    try {
      await apiRequest('POST', `/sessions/${id}/model`, { model: modelId });
      updateView(set, id, (v) => ({ ...v, meta: { ...v.meta, model: modelId } }));
    } catch (err) {
      console.error('Failed to switch model', err);
    }
  },

  setDensity: (id, density) => updateView(set, id, (v) => ({ ...v, density })),

  togglePane: (id, pane) =>
    updateView(set, id, (v) => {
      const has = v.panes.includes(pane);
      const panes = has ? v.panes.filter((p) => p !== pane) : [...v.panes, pane];
      return { ...v, panes: panes.length ? panes : ['chat'], activePane: has ? v.activePane : pane };
    }),

  refreshDiff: async (id) => {
    const view = get().sessions[id];
    if (!view || !view.meta.cwd) return;
    try {
      const resp = await apiRequest<{ files: GitFileDiff[] }>('GET', `/sessions/${id}/diff`);
      updateView(set, id, (v) => ({ ...v, diffs: resp.files || [] }));
    } catch (err) {
      console.error('Failed to fetch diff', err);
    }
  },

  openFile: async (id, path) => {
    try {
      const resp = await apiRequest<{ content: string }>('GET', `/sessions/${id}/file?path=${encodeURIComponent(path)}`);
      updateView(set, id, (v) => ({ ...v, openFile: { path, content: resp.content }, activePane: 'file' }));
    } catch (err) {
      console.error('Failed to read file', err);
    }
  },

  saveFile: async (id, path, content) => {
    try {
      console.log('[saveFile] Saving file:', { id, path, contentLength: content.length });
      await apiRequest<{ status: string }>('PUT', `/sessions/${id}/file?path=${encodeURIComponent(path)}`, { content });
      console.log('[saveFile] File saved successfully');
      // Update the view with new content
      updateView(set, id, (v) => ({
        ...v,
        openFile: v.openFile?.path === path ? { path, content } : v.openFile,
      }));
      return true;
    } catch (err) {
      console.error('Failed to save file', err);
      return false;
    }
  },
}));

// ── Helpers ───────────────────────────────────────────────────────────────

type SetFn = (partial: Partial<StoreState> | ((s: StoreState) => Partial<StoreState>)) => void;

function updateView(
  setFn: SetFn,
  id: string,
  fn: (v: SessionView) => Partial<SessionView>,
): void {
  setFn((s) => {
    const v = s.sessions[id];
    if (!v) return {};
    return { sessions: { ...s.sessions, [id]: { ...v, ...fn(v) } } };
  });
}

function inferToolKind(name: string): AcpToolKind {
  const lower = name.toLowerCase();
  // KB agent tools — must check before generic patterns
  if (lower === 'kb_search' || lower === 'kb_list' || lower === 'kb_maintain') return 'search';
  if (lower === 'kb_read') return 'read';
  if (lower === 'kb_save') return 'edit';
  if (lower.includes('read') || lower.includes('cat') || lower.includes('view')) return 'read';
  if (lower.includes('edit') || lower.includes('write') || lower.includes('replace')) return 'edit';
  if (lower.includes('delete') || lower.includes('remove') || lower.includes('rm')) return 'delete';
  if (lower.includes('move') || lower.includes('rename')) return 'move';
  if (lower.includes('search') || lower.includes('grep') || lower.includes('glob') || lower.includes('find')) return 'search';
  if (lower.includes('bash') || lower.includes('exec') || lower.includes('shell') || lower.includes('run')) return 'execute';
  if (lower.includes('think') || lower.includes('reason')) return 'think';
  if (lower.includes('fetch') || lower.includes('http') || lower.includes('web')) return 'fetch';
  return 'other';
}

function projectName(cwd: string): string {
  const parts = cwd.replace(/[\\/]+$/, '').split(/[\\/]/);
  return parts[parts.length - 1] || cwd;
}
