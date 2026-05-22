// modelStore.ts — Model list and switching.
import { create } from 'zustand';
import * as api from '../services/api';

export interface Model {
  id: string;
  provider: string;
  name: string;
}

interface ModelState {
  models: Model[];
  currentModel: { provider: string; model: string } | null;
  loading: boolean;

  loadModels: () => Promise<void>;
  switchModel: (sessionId: string, modelId: string, provider?: string) => Promise<void>;
  setCurrentModel: (provider: string, model: string) => void;
}

export const useModelStore = create<ModelState>((set) => ({
  models: [],
  currentModel: null,
  loading: false,

  loadModels: async () => {
    set({ loading: true });
    try {
      const res = await api.listModels();
      const models: Model[] = res.models.map((m) => ({
        id: m.id,
        provider: m.provider,
        name: m.name,
      }));
      const currentModel = res.current
        ? { provider: res.current.provider, model: res.current.id }
        : null;
      set({ models, currentModel });
    } catch (err) {
      console.error('Failed to load models', err);
    } finally {
      set({ loading: false });
    }
  },

  switchModel: async (sessionId: string, modelId: string, provider?: string) => {
    const result = await api.switchModel(sessionId, modelId, provider);
    set({ currentModel: { provider: result.provider, model: result.model } });
  },

  setCurrentModel: (provider: string, model: string) => {
    set({ currentModel: { provider, model } });
  },
}));
