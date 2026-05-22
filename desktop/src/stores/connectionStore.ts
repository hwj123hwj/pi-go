// connectionStore.ts — WebSocket connection state.
import { create } from 'zustand';
import { wsService } from '../services/websocket';

interface ConnectionState {
  connected: boolean;
  serverUrl: string;
  connect: (url: string) => void;
  disconnect: () => void;
}

let listenersSetup = false;

export const useConnectionStore = create<ConnectionState>((set) => ({
  connected: false,
  serverUrl: '',

  connect: (url: string) => {
    set({ serverUrl: url });

    // Only register event listeners once
    if (!listenersSetup) {
      listenersSetup = true;
      wsService.on('connected', () => set({ connected: true }));
      wsService.on('disconnected', () => set({ connected: false }));
    }

    wsService.connect(url);
  },

  disconnect: () => {
    wsService.disconnect();
    set({ connected: false });
  },
}));
