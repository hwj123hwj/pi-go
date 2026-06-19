/**
 * store.ts — Single source of truth for the pi-go desktop renderer.
 *
 * Talks directly to the pi-go backend via REST (fetch) + WebSocket.
 * No IPC bridge — the renderer hits the HTTP server managed by Electron's
 * pi-go-manager directly.
 */

import { create } from 'zustand';
import { type Lang, loadStoredLang, persistLang } from './i18n/i18n';
import { type ThemeMode, loadStoredTheme, persistTheme } from './theme';
import { deriveTitleFromMessage } from './sessionTitle';
import type {
  AcpToolKind,
  DesktopSessionEvent,
  ModelInfo,
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

function getBaseUrl(): string {
  return baseUrl;
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
      this.emit('connected', {});
    };

    this.ws.onclose = () => {
      console.log('[ws] Disconnected');
      this._connected = false;
      this.emit('disconnected', {});
      // Auto-reconnect
      setTimeout(() => this.connect(url), 2000);
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
    };

export interface SessionView {
  meta: SessionMeta;
  transcript: ChatItem[];
  density: ViewDensity;
  draftAssistantId?: string;
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

  init: () => Promise<void>;
  refreshSessions: () => Promise<void>;
  setActive: (id: string) => void;
  createSession: (opts?: { cwd?: string; model?: string }) => Promise<string>;
  deleteSession: (id: string) => Promise<void>;
  sendPrompt: (id: string, text: string) => Promise<void>;
  cancel: (id: string) => Promise<void>;
  setModel: (id: string, modelId: string) => Promise<void>;
  setDensity: (id: string, density: ViewDensity) => void;
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
  return { meta, transcript: [], density: 'normal' };
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

export const useStore = create<StoreState>((set, get) => ({
  ready: false,
  connected: false,
  sessions: {},
  order: [],
  models: [],
  currentModel: undefined,
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

  update: null,
  checkUpdate: async () => {
    try {
      const info = await window.piAPI?.checkForUpdate();
      if (info) {
        set({ update: { supported: true, phase: 'available', info, currentVersion: '0.2.0' } });
      } else {
        set({ update: { supported: true, phase: 'idle', info: null, currentVersion: '0.2.0' } });
      }
    } catch {
      set({ update: { supported: true, phase: 'error', error: 'Check failed', currentVersion: '0.2.0' } });
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
          transcript[transcript.length - 1] = { ...last, text: last.text + delta };
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
      const resultText = typeof result === 'string' ? result : JSON.stringify(result || '');
      updateView(set, sessionId, (v) => {
        const transcript = v.transcript.map((item) => {
          if (item.kind === 'tool' && item.toolCallId === toolCallId) {
            return {
              ...item,
              status: (isError ? 'failed' : 'completed') as ToolCallStatus,
              content: [{ text: resultText }],
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
          const meta: SessionMeta = {
            id: sess.id,
            title: s.sessions[sess.id]?.meta.title || `Session ${sess.id.slice(-6)}`,
            cwd,
            status: 'idle' as SessionRunStatus,
            model: s.sessions[sess.id]?.meta.model,
            availableModels: defaultModels(),
            createdAt: sess.created_at || 0,
            updatedAt: sess.last_active || 0,
          };
          newSessions[sess.id] = s.sessions[sess.id] ?? emptyView(meta);
          // Update meta for existing sessions (in case cwd was loaded from backend)
          if (s.sessions[sess.id]) {
            newSessions[sess.id] = { ...s.sessions[sess.id], meta: { ...s.sessions[sess.id].meta, cwd, updatedAt: sess.last_active || 0 } };
          }
        }
        return { sessions: newSessions, order: sessions.map((s) => s.id) };
      });
    } catch (err) {
      console.error('Failed to load sessions', err);
    }
  },

  setActive: (id) => set({ activeSessionId: id }),

  createSession: async (opts) => {
    const body: Record<string, string> = {};
    if (opts?.cwd) body.cwd = opts.cwd;
    if (opts?.model) body.model = opts.model;
    const result = await apiRequest<{ id: string; created_at: number }>('POST', '/sessions', body);
    const meta: SessionMeta = {
      id: result.id,
      title: opts?.cwd ? projectName(opts.cwd) : 'New Session',
      cwd: opts?.cwd || '',
      status: 'idle' as SessionRunStatus,
      model: opts?.model || cachedCurrentModel,
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
