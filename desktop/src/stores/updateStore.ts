// updateStore.ts — Update detection state management.
import { create } from 'zustand';

export interface UpdateInfo {
  version: string;
  downloadUrl: string;
  releaseNotes: string;
}

interface UpdateState {
  updateInfo: UpdateInfo | null;
  checking: boolean;
  dismissed: boolean;

  checkForUpdate: () => Promise<void>;
  dismissUpdate: () => void;
}

export const useUpdateStore = create<UpdateState>((set) => ({
  updateInfo: null,
  checking: false,
  dismissed: false,

  checkForUpdate: async () => {
    // Only works in Electron (packaged or dev mode with piAPI)
    if (!window.piAPI?.checkForUpdate) {
      return;
    }

    set({ checking: true });
    try {
      const info = await window.piAPI.checkForUpdate();
      if (info) {
        set({ updateInfo: info, dismissed: false });
      }
    } catch {
      // Silent fail — network errors should not disrupt the user
    } finally {
      set({ checking: false });
    }
  },

  dismissUpdate: () => {
    set({ dismissed: true });
  },
}));
