/**
 * store.ts — Zustand store for Pi-Go mobile
 *
 * Manages sessions, active chat transcript, WebSocket events, models.
 * Adapted from desktop/src/store.ts, stripped of desktop-only features
 * (workspace panels, diffs, file editing).
 */

import { create } from 'zustand';
import { apiRequest, setBaseUrl, getBaseUrl, loadStoredServerUrl } from '../api';
import { wsService } from '../api/ws';
import type { ChatItem, ModelInfo, SessionMeta, SessionView } from '../types';

interface StoreState {
  ready: boolean;
  connected: boolean;
  serverReady: boolean;

  sessions: Record<string, SessionView>;
  order: string[];
  activeSessionId: string | undefined;

  models: ModelInfo[];

  // Actions
  init: () => Promise<void>;
  refreshSessions: () => Promise<void>;
  setActive: (id: string) => Promise<void>;
  createSession: (opts?: { cwd?: string; application?: string; model?: string }) => Promise<string>;
  deleteSession: (id: string) => Promise<void>;
  sendPrompt: (id: string, text: string) => Promise<void>;
  cancel: (id: string) => Promise<void>;
  setModel: (id: string, modelId: string) => Promise<void>;
}

let initialized = false;

function newId(): string {
  return Math.random().toString(36).slice(2, 10);
}

function deriveTitle(text: string): string {
  const trimmed = text.trim().replace(/\n/g, ' ');
  return trimmed.length > 40 ? trimmed.slice(0, 40) + '…' : trimmed;
}

function emptyView(meta: SessionMeta): SessionView {
  return { meta, transcript: [] };
}

export const useStore = create<StoreState>((set, get) => ({
  ready: false,
  connected: false,
  serverReady: false,
  sessions: {},
  order: [],
  activeSessionId: undefined,
  models: [],

  init: async () => {
    if (initialized) return;
    initialized = true;

    // Load stored server URL
    const stored = await loadStoredServerUrl();
    if (stored) {
      setBaseUrl(stored);
    } else {
      set({ ready: false, serverReady: false });
      return;
    }

    set({ serverReady: true });

    // Connect WebSocket
    wsService.connect(getBaseUrl());

    wsService.on('connected', () => set({ connected: true }));
    wsService.on('disconnected', () => set({ connected: false }));

    // ── WebSocket event handlers ──

    wsService.on('type:status', (data: any) => {
      const sid = data.session_id;
      if (!sid) return;
      if (!data.streaming) {
        updateView(set, sid, (v) => ({
          ...v,
          meta: { ...v.meta, status: 'idle' },
        }));
      }
    });

    wsService.on('event:text_delta', (data: any) => {
      const sid = data.session_id;
      const delta = data.event?.text_delta || '';
      if (!sid || !delta) return;
      updateView(set, sid, (v) => {
        const transcript = [...v.transcript];
        const last = transcript[transcript.length - 1];
        if (last && last.kind === 'assistant') {
          transcript[transcript.length - 1] = { ...last, text: last.text + delta };
        } else {
          transcript.push({ kind: 'assistant', id: newId(), text: delta });
        }
        return { ...v, transcript, meta: { ...v.meta, status: 'thinking' } };
      });
    });

    wsService.on('event:tool_start', (data: any) => {
      const sid = data.session_id;
      if (!sid) return;
      const toolName = data.event?.tool_name || 'tool';
      const item: ChatItem = {
        kind: 'tool',
        id: newId(),
        toolCallId: data.event?.tool_call_id || newId(),
        title: toolName,
        toolKind: toolName,
        status: 'in_progress',
        text: '',
      };
      updateView(set, sid, (v) => ({
        ...v,
        transcript: [...v.transcript, item],
      }));
    });

    wsService.on('event:tool_end', (data: any) => {
      const sid = data.session_id;
      if (!sid) return;
      const toolCallId = data.event?.tool_call_id || '';
      const isError = data.event?.is_error || false;
      const result = data.event?.tool_result;
      const resultText =
        typeof result === 'string' ? result :
        result?.UserFacing || result?.Content || JSON.stringify(result || '');

      updateView(set, sid, (v) => ({
        ...v,
        transcript: v.transcript.map((item) =>
          item.kind === 'tool' && item.toolCallId === toolCallId
            ? { ...item, status: isError ? 'failed' : 'completed', text: resultText }
            : item
        ),
      }));
    });

    wsService.on('event:error', (data: any) => {
      const sid = data.session_id;
      if (!sid) return;
      const error = data.event?.error || 'Unknown error';
      updateView(set, sid, (v) => ({
        ...v,
        transcript: [...v.transcript, { kind: 'error', id: newId(), text: error }],
        meta: { ...v.meta, status: 'error' },
      }));
    });

    // Fetch models
    try {
      const rawModels = await apiRequest<any[]>('GET', '/models');
      const models: ModelInfo[] = (rawModels || []).map((m) => ({
        modelId: m.model_id || m.id,
        name: m.name || m.model_id || m.id,
      }));
      set({ models });
    } catch {
      // models are optional
    }

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
          const existing = s.sessions[sess.id];
          const meta: SessionMeta = {
            id: sess.id,
            title: sess.title || existing?.meta.title || `Session ${sess.id.slice(-6)}`,
            cwd: sess.workspace || existing?.meta.cwd || '',
            status: existing?.meta.status || 'idle',
            model: existing?.meta.model,
            application: sess.application || existing?.meta.application,
            createdAt: sess.created_at || 0,
            updatedAt: sess.last_active || 0,
          };
          newSessions[sess.id] = existing ?? emptyView(meta);
          if (existing) {
            newSessions[sess.id] = { ...existing, meta: { ...existing.meta, ...meta } };
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
            if (msg.thinking) {
              items.push({ kind: 'thought', id: newId(), text: msg.thinking });
            }
            if (msg.tool_calls) {
              for (const tc of msg.tool_calls) {
                const resultMsg = messages.find(
                  (m: any) => m.role === 'tool' && m.tool_call_id === tc.id,
                );
                items.push({
                  kind: 'tool',
                  id: newId(),
                  toolCallId: tc.id,
                  title: tc.name,
                  toolKind: tc.name,
                  status: resultMsg?.is_error ? 'failed' : 'completed',
                  text: resultMsg?.content || '',
                });
              }
            }
            if (msg.content) {
              items.push({ kind: 'assistant', id: newId(), text: msg.content });
            }
          }
        }

        if (items.length > 0) {
          updateView(set, id, (v) => ({ ...v, transcript: items }));
        }
      } catch (err) {
        console.error('Failed to load transcript', err);
      }
    }
  },

  createSession: async (opts) => {
    const body: Record<string, string> = {};
    if (opts?.cwd) body.cwd = opts.cwd;
    if (opts?.application) body.application = opts.application;
    const result = await apiRequest<{ id: string; created_at: number }>('POST', '/sessions', body);
    const meta: SessionMeta = {
      id: result.id,
      title: opts?.cwd ? opts.cwd.split(/[\\/]/).pop() || 'Session' : 'New Chat',
      cwd: opts?.cwd || '',
      status: 'idle',
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
    const userItem: ChatItem = { kind: 'user', id: newId(), text };
    const assistantItem: ChatItem = { kind: 'assistant', id: newId(), text: '' };
    updateView(set, id, (v) => ({
      ...v,
      transcript: [...v.transcript, userItem, assistantItem],
      meta: { ...v.meta, status: 'thinking' },
    }));
    wsService.send({ type: 'prompt', session_id: id, prompt: text });
  },

  cancel: async (id) => {
    wsService.send({ type: 'cancel', session_id: id });
    updateView(set, id, (v) => ({ ...v, meta: { ...v.meta, status: 'idle' } }));
  },

  setModel: async (id, modelId) => {
    try {
      await apiRequest('POST', `/sessions/${id}/model`, { model: modelId });
      updateView(set, id, (v) => ({ ...v, meta: { ...v.meta, model: modelId } }));
    } catch (err) {
      console.error('Failed to switch model', err);
    }
  },
}));

// Helper: update a single session view
function updateView(
  set: (fn: (s: StoreState) => Partial<StoreState>) => void,
  sessionId: string,
  updater: (v: SessionView) => Partial<SessionView>,
): void {
  set((s) => {
    const view = s.sessions[sessionId];
    if (!view) return {};
    return { sessions: { ...s.sessions, [sessionId]: { ...view, ...updater(view) } } };
  });
}
