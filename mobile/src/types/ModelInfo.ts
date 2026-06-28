export type SessionRunStatus = 'idle' | 'starting' | 'thinking' | 'error' | 'exited';

export interface ModelInfo {
  modelId: string;
  name: string;
}
