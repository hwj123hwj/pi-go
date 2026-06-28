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
