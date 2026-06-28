import type { ChatItem } from './ChatItem';

export interface SessionMeta {
  id: string;
  title: string;
  cwd: string;
  status: import('./ModelInfo').SessionRunStatus;
  model?: string;
  application?: string;
  createdAt: number;
  updatedAt: number;
}

export interface SessionView {
  meta: SessionMeta;
  transcript: ChatItem[];
}
