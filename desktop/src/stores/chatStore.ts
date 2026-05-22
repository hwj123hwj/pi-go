// chatStore.ts — Chat messages and streaming state.
import { create } from 'zustand';
import { wsService } from '../services/websocket';
import * as api from '../services/api';

// ─── Types ───────────────────────────────────────────────────────────────────

export interface Message {
  id: string;
  role: 'user' | 'assistant';
  text: string;
  toolCalls: ToolCall[];
  timestamp: number;
  streaming?: boolean; // Is this message still being streamed?
}

export interface ToolCall {
  id: string;
  name: string;
  status: 'running' | 'done' | 'error';
  result?: string;
}

// ─── Store ───────────────────────────────────────────────────────────────────

interface ChatState {
  messagesBySession: Record<string, Message[]>;
  streamingSessionId: string | null;
  error: string | null;

  // Load history for a session
  loadHistory: (sessionId: string) => Promise<void>;

  // Send a prompt via WebSocket
  sendPrompt: (sessionId: string, prompt: string) => void;

  // Cancel current generation
  cancelGeneration: () => void;

  // Clear error
  clearError: () => void;

  // Setup WebSocket listeners
  setupWSListeners: () => void;

  // Internal mutations
  _addUserMessage: (sessionId: string, text: string) => void;
  _startAssistantMessage: (sessionId: string) => void;
  _appendDelta: (sessionId: string, delta: string) => void;
  _addToolStart: (sessionId: string, toolName: string, toolCallId: string) => void;
  _addToolEnd: (sessionId: string, toolCallId: string, result: any, isError: boolean) => void;
  _finalizeMessage: (sessionId: string) => void;
  _setError: (sessionId: string, error: string) => void;
}

let msgCounter = 0;
let wsListenersSetup = false;
function nextId(): string {
  return `msg_${Date.now()}_${++msgCounter}`;
}

export const useChatStore = create<ChatState>((set, get) => ({
  messagesBySession: {},
  streamingSessionId: null,
  error: null,

  loadHistory: async (sessionId: string) => {
    try {
      const raw = await api.getSessionMessages(sessionId);
      const messages: any[] = Array.isArray(raw) ? raw : [];
      // Only show user and assistant messages, skip tool_result
      const mapped: Message[] = messages
        .filter((m: any) => m.role === 'user' || m.role === 'assistant')
        .map((m: any) => ({
          id: nextId(),
          role: m.role === 'user' ? 'user' as const : 'assistant' as const,
          text: m.content || '',
          toolCalls: [],
          timestamp: Date.now(),
        }));
      set((state) => ({
        messagesBySession: {
          ...state.messagesBySession,
          [sessionId]: mapped,
        },
      }));
    } catch (err) {
      console.error('Failed to load history', err);
    }
  },

  sendPrompt: (sessionId: string, prompt: string) => {
    const state = get();
    if (state.streamingSessionId) {
      return; // Already streaming
    }

    // Add user message locally
    get()._addUserMessage(sessionId, prompt);

    // Add placeholder assistant message
    get()._startAssistantMessage(sessionId);

    // Send via WebSocket
    wsService.send({
      type: 'prompt',
      session_id: sessionId,
      prompt,
    });

    set({ streamingSessionId: sessionId, error: null });
  },

  cancelGeneration: () => {
    const { streamingSessionId } = get();
    if (streamingSessionId) {
      wsService.send({
        type: 'cancel',
        session_id: streamingSessionId,
      });
      // Finalize the current streaming message
      get()._finalizeMessage(streamingSessionId);
      set({ streamingSessionId: null });
    }
  },

  clearError: () => set({ error: null }),

  setupWSListeners: () => {
    // Prevent duplicate registration (React StrictMode calls useEffect twice in dev)
    if (wsListenersSetup) return;
    wsListenersSetup = true;

    // Handle session_id response
    wsService.on('type:session_id', (data: any) => {
      // Server may have auto-created a session; update if needed
    });

    // Handle status updates
    wsService.on('type:status', (data: any) => {
      if (!data.streaming) {
        // Streaming done
        const sessionId = data.session_id;
        if (sessionId) {
          get()._finalizeMessage(sessionId);
        }
        set({ streamingSessionId: null });
      }
    });

    // Handle text_delta events
    wsService.on('event:text_delta', (data: any) => {
      const sessionId = data.session_id;
      const delta = data.event?.text_delta || '';
      if (sessionId && delta) {
        get()._appendDelta(sessionId, delta);
      }
    });

    // Handle tool_start events
    wsService.on('event:tool_start', (data: any) => {
      const sessionId = data.session_id;
      const toolName = data.event?.tool_name || '';
      const toolCallId = data.event?.tool_call_id || '';
      if (sessionId) {
        get()._addToolStart(sessionId, toolName, toolCallId);
      }
    });

    // Handle tool_end events
    wsService.on('event:tool_end', (data: any) => {
      const sessionId = data.session_id;
      const toolCallId = data.event?.tool_call_id || '';
      const result = data.event?.tool_result;
      const isError = data.event?.is_error || false;
      if (sessionId) {
        get()._addToolEnd(sessionId, toolCallId, result, isError);
      }
    });

    // Handle turn_end events
    wsService.on('event:turn_end', () => {
      // Turn ended, but streaming may continue (more tool calls)
    });

    // Handle done events
    wsService.on('event:done', () => {
      // Final done
    });

    // Handle error events
    wsService.on('event:error', (data: any) => {
      const sessionId = data.session_id;
      const error = data.event?.error || 'Unknown error';
      if (sessionId) {
        get()._setError(sessionId, error);
      }
    });

    // Handle model_info from switch_model
    wsService.on('type:model_info', (data: any) => {
      // Model was switched via WebSocket
    });

    // Handle error messages
    wsService.on('type:error', (data: any) => {
      set({ error: data.message || 'Unknown error' });
      set({ streamingSessionId: null });
    });
  },

  // ─── Internal mutations ──────────────────────────────────────────────────────

  _addUserMessage: (sessionId: string, text: string) => {
    const msg: Message = {
      id: nextId(),
      role: 'user',
      text,
      toolCalls: [],
      timestamp: Date.now(),
    };
    set((state) => ({
      messagesBySession: {
        ...state.messagesBySession,
        [sessionId]: [...(state.messagesBySession[sessionId] || []), msg],
      },
    }));
  },

  _startAssistantMessage: (sessionId: string) => {
    const msg: Message = {
      id: nextId(),
      role: 'assistant',
      text: '',
      toolCalls: [],
      timestamp: Date.now(),
      streaming: true,
    };
    set((state) => ({
      messagesBySession: {
        ...state.messagesBySession,
        [sessionId]: [...(state.messagesBySession[sessionId] || []), msg],
      },
    }));
  },

  _appendDelta: (sessionId: string, delta: string) => {
    set((state) => {
      const messages = state.messagesBySession[sessionId] || [];
      if (messages.length === 0) return state;

      const lastMsg = messages[messages.length - 1];
      if (lastMsg.role !== 'assistant') return state;

      const updated = [...messages];
      updated[updated.length - 1] = {
        ...lastMsg,
        text: lastMsg.text + delta,
      };

      return {
        messagesBySession: {
          ...state.messagesBySession,
          [sessionId]: updated,
        },
      };
    });
  },

  _addToolStart: (sessionId: string, toolName: string, toolCallId: string) => {
    set((state) => {
      const messages = state.messagesBySession[sessionId] || [];
      if (messages.length === 0) return state;

      const lastMsg = messages[messages.length - 1];
      if (lastMsg.role !== 'assistant') return state;

      const tc: ToolCall = {
        id: toolCallId || nextId(),
        name: toolName,
        status: 'running',
      };

      const updated = [...messages];
      updated[updated.length - 1] = {
        ...lastMsg,
        toolCalls: [...lastMsg.toolCalls, tc],
      };

      return {
        messagesBySession: {
          ...state.messagesBySession,
          [sessionId]: updated,
        },
      };
    });
  },

  _addToolEnd: (sessionId: string, toolCallId: string, result: any, isError: boolean) => {
    set((state) => {
      const messages = state.messagesBySession[sessionId] || [];
      if (messages.length === 0) return state;

      const lastMsg = messages[messages.length - 1];
      if (lastMsg.role !== 'assistant') return state;

      const toolCalls = lastMsg.toolCalls.map((tc) =>
        tc.id === toolCallId
          ? { ...tc, status: isError ? ('error' as const) : ('done' as const), result: String(result || '') }
          : tc
      );

      const updated = [...messages];
      updated[updated.length - 1] = {
        ...lastMsg,
        toolCalls,
      };

      return {
        messagesBySession: {
          ...state.messagesBySession,
          [sessionId]: updated,
        },
      };
    });
  },

  _finalizeMessage: (sessionId: string) => {
    set((state) => {
      const messages = state.messagesBySession[sessionId] || [];
      if (messages.length === 0) return state;

      const updated = [...messages];
      const last = updated[updated.length - 1];
      if (last && last.role === 'assistant') {
        updated[updated.length - 1] = { ...last, streaming: false };
      }

      return {
        messagesBySession: {
          ...state.messagesBySession,
          [sessionId]: updated,
        },
        streamingSessionId: null,
      };
    });
  },

  _setError: (sessionId: string, error: string) => {
    set({ error, streamingSessionId: null });
  },
}));
