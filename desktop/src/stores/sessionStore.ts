// sessionStore.ts — Session management.
import { create } from 'zustand';
import * as api from '../services/api';

export interface Session {
  id: string;
  messageCount: number;
  lastActive: number;
  createdAt: number;
  provider: string;
  model: string;
}

interface SessionState {
  sessions: Session[];
  currentSessionId: string;
  loading: boolean;

  loadSessions: () => Promise<void>;
  createSession: () => Promise<string>;
  switchSession: (id: string) => void;
  deleteSession: (id: string) => Promise<void>;
  setCurrentSessionId: (id: string) => void;
}

export const useSessionStore = create<SessionState>((set, get) => ({
  sessions: [],
  currentSessionId: '',
  loading: false,

  loadSessions: async () => {
    set({ loading: true });
    try {
      const raw = await api.listSessions();
      const sessions: any[] = Array.isArray(raw) ? raw : [];
      const mapped: Session[] = sessions.map((s: any) => ({
        id: s.id,
        messageCount: s.message_count || 0,
        lastActive: s.last_active || 0,
        createdAt: s.created_at || 0,
        provider: '',
        model: '',
      }));
      set({ sessions: mapped });

      // Auto-select first session if none selected
      if (!get().currentSessionId && mapped.length > 0) {
        set({ currentSessionId: mapped[0].id });
      }
    } catch (err) {
      console.error('Failed to load sessions', err);
    } finally {
      set({ loading: false });
    }
  },

  createSession: async () => {
    const result = await api.createSession();
    const newSession: Session = {
      id: result.id,
      messageCount: 0,
      lastActive: Date.now(),
      createdAt: result.created_at || Date.now(),
      provider: '',
      model: '',
    };
    set((state) => ({
      sessions: [newSession, ...state.sessions],
      currentSessionId: result.id,
    }));
    return result.id;
  },

  switchSession: async (id: string) => {
    set({ currentSessionId: id });
    // Sync model state from backend for this session
    try {
      const info = await api.getSessionInfo(id);
      if (info.provider || info.model) {
        const { useModelStore } = await import('./modelStore');
        useModelStore.getState().setCurrentModel(info.provider || '', info.model || '');
      }
    } catch {
      // Session info fetch failed — keep current model state
    }
  },

  deleteSession: async (id: string) => {
    await api.deleteSession(id);
    set((state) => {
      const sessions = state.sessions.filter((s) => s.id !== id);
      const currentSessionId =
        state.currentSessionId === id
          ? sessions.length > 0
            ? sessions[0].id
            : ''
          : state.currentSessionId;
      return { sessions, currentSessionId };
    });
  },

  setCurrentSessionId: (id: string) => {
    set({ currentSessionId: id });
  },
}));
