/**
 * types.ts — Shared types for Pi-Go mobile (React Native / Expo)
 */

export type SessionRunStatus = 'idle' | 'starting' | 'thinking' | 'error' | 'exited';

export interface ModelInfo {
  modelId: string;
  name: string;
}

export interface SessionMeta {
  id: string;
  title: string;
  cwd: string;
  status: SessionRunStatus;
  model?: string;
  application?: string;
  createdAt: number;
  updatedAt: number;
}

export type ChatItemKind = 'user' | 'assistant' | 'tool' | 'thought' | 'error';

export interface ChatItem {
  kind: ChatItemKind;
  id: string;
  text: string;
  // Tool-specific
  toolCallId?: string;
  title?: string;
  toolKind?: string;
  status?: string;
  details?: string;
}

export interface SessionView {
  meta: SessionMeta;
  transcript: ChatItem[];
}
