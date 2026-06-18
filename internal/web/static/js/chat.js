// Chat panel: messages, streaming, tool calls, input

import { renderMarkdown } from './markdown.js';

const TOOL_ICONS = {
  bash: '⌨️', read: '📖', write: '✏️', edit: '📝',
  grep: '🔍', find: '📂', ls: '📋',
};

export class ChatPanel {
  constructor(ws, state) {
    this.ws = ws;
    this.state = state;
    this.messageList = document.getElementById('message-list');
    this.input = document.getElementById('message-input');
    this.sendBtn = document.getElementById('send-btn');
    this.stopBtn = document.getElementById('stop-btn');
    this.emptyState = document.getElementById('empty-state');
    this.chatView = document.getElementById('chat-view');

    this._bindEvents();
    this._bindWS();
  }

  _bindEvents() {
    // Send on button click
    this.sendBtn.addEventListener('click', () => this._send());

    // Send on Enter (Shift+Enter for newline)
    this.input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        this._send();
      }
    });

    // Stop on button click
    this.stopBtn.addEventListener('click', () => {
      if (this.state.currentSessionId) {
        this.ws.sendCancel(this.state.currentSessionId);
      }
    });

    // Auto-resize textarea
    this.input.addEventListener('input', () => {
      this.input.style.height = 'auto';
      this.input.style.height = Math.min(this.input.scrollHeight, 200) + 'px';
    });
  }

  _bindWS() {
    this.ws.on('event:text_delta', (event, sessionId) => {
      if (sessionId !== this.state.currentSessionId) return;
      this._appendStreamText(event.text_delta);
    });

    this.ws.on('event:tool_start', (event, sessionId) => {
      if (sessionId !== this.state.currentSessionId) return;
      this._addToolCall(event.tool_call_id, event.tool_name, 'running');
    });

    this.ws.on('event:tool_end', (event, sessionId) => {
      if (sessionId !== this.state.currentSessionId) return;
      this._updateToolCall(event.tool_call_id, event.is_error ? 'error' : 'done', event.tool_result);
    });

    this.ws.on('event:done', (event, sessionId) => {
      if (sessionId !== this.state.currentSessionId) return;
      this._finalizeStream(event.final_message);
    });

    this.ws.on('event:error', (event, sessionId) => {
      if (sessionId !== this.state.currentSessionId) return;
      this._finalizeWithError(event.error);
    });

    this.ws.on('status', (data) => {
      if (data.session_id !== this.state.currentSessionId) return;
      this.state.streaming = data.streaming;
      this._updateButtons();
    });
  }

  show() {
    this.emptyState.style.display = 'none';
    this.chatView.style.display = 'flex';
    this.input.focus();
  }

  hide() {
    this.emptyState.style.display = 'flex';
    this.chatView.style.display = 'none';
  }

  clear() {
    this.messageList.innerHTML = '';
    this._currentStreamEl = null;
    this._currentStreamText = '';
    this._currentToolCalls = {};
  }

  async loadHistory(sessionId) {
    try {
      const resp = await fetch(`${this.state.baseUrl}/sessions/${sessionId}/messages`);
      if (!resp.ok) return;
      const messages = await resp.json();
      this.clear();
      messages.forEach(msg => {
        if (msg.role === 'user') {
          this._addUserMessage(msg.content);
        } else if (msg.role === 'assistant') {
          this._addAssistantMessage(msg.content);
        }
      });
      this._scrollToBottom();
    } catch (e) {
      console.error('Failed to load history:', e);
    }
  }

  _send() {
    const text = this.input.value.trim();
    if (!text || !this.state.currentSessionId) return;

    this._addUserMessage(text);
    this._startAssistantStream();
    this.ws.sendPrompt(this.state.currentSessionId, text);

    this.input.value = '';
    this.input.style.height = 'auto';
    this.state.streaming = true;
    this._updateButtons();
    this._scrollToBottom();
  }

  _addUserMessage(text) {
    const el = document.createElement('div');
    el.className = 'user-message';
    el.innerHTML = `
      <div class="user-bubble">${this._escapeHtml(text)}</div>
      <div class="user-avatar">U</div>
    `;
    this.messageList.appendChild(el);
    this._scrollToBottom();
  }

  _addAssistantMessage(text) {
    const el = document.createElement('div');
    el.className = 'assistant-message';
    el.innerHTML = `
      <div class="assistant-avatar">Pi</div>
      <div class="assistant-content">
        <div class="markdown-body">${renderMarkdown(text)}</div>
      </div>
    `;
    this.messageList.appendChild(el);
    this._scrollToBottom();
  }

  _startAssistantStream() {
    const el = document.createElement('div');
    el.className = 'assistant-message';
    el.innerHTML = `
      <div class="assistant-avatar">Pi</div>
      <div class="assistant-content">
        <div class="markdown-body stream-text"></div>
        <div class="tool-calls-container"></div>
      </div>
    `;
    this.messageList.appendChild(el);
    this._currentStreamEl = el;
    this._currentStreamText = '';
    this._currentToolCalls = {};
    this._scrollToBottom();
  }

  _appendStreamText(delta) {
    if (!this._currentStreamEl) return;
    this._currentStreamText += delta;
    const textEl = this._currentStreamEl.querySelector('.stream-text');
    textEl.innerHTML = renderMarkdown(this._currentStreamText) + '<span class="streaming-cursor"></span>';
    this._scrollToBottom();
  }

  _addToolCall(toolCallId, toolName, status) {
    if (!this._currentStreamEl) return;
    const container = this._currentStreamEl.querySelector('.tool-calls-container');
    const icon = TOOL_ICONS[toolName] || '🔧';

    const el = document.createElement('div');
    el.className = `tool-call ${status}`;
    el.dataset.toolId = toolCallId;
    el.innerHTML = `
      <div class="tool-call-header">
        <span class="tool-toggle">▶</span>
        <span class="tool-icon">${icon}</span>
        <span class="tool-name">${this._escapeHtml(toolName)}</span>
        <span class="tool-status">
          ${status === 'running' ? '<span class="tool-spinner"></span>' : ''}
        </span>
      </div>
      <pre class="tool-result"><code></code></pre>
    `;

    // Toggle expand/collapse
    el.querySelector('.tool-call-header').addEventListener('click', () => {
      el.classList.toggle('expanded');
    });

    container.appendChild(el);
    this._currentToolCalls[toolCallId] = el;
    this._scrollToBottom();
  }

  _updateToolCall(toolCallId, status, result) {
    const el = this._currentToolCalls[toolCallId];
    if (!el) return;

    el.className = `tool-call ${status}`;
    const statusEl = el.querySelector('.tool-status');
    if (status === 'done') {
      statusEl.innerHTML = '<span class="tool-check">✓</span>';
    } else if (status === 'error') {
      statusEl.innerHTML = '<span class="tool-error">✗</span>';
    }

    if (result) {
      const codeEl = el.querySelector('.tool-result code');
      const resultText = typeof result === 'string' ? result : JSON.stringify(result, null, 2);
      codeEl.textContent = resultText.slice(0, 3000) + (resultText.length > 3000 ? '\n... (truncated)' : '');
    }

    this._scrollToBottom();
  }

  _finalizeStream(finalMessage) {
    if (!this._currentStreamEl) return;

    // Remove streaming cursor
    const cursor = this._currentStreamEl.querySelector('.streaming-cursor');
    if (cursor) cursor.remove();

    // If we have a final message, render it properly
    if (finalMessage?.text && !this._currentStreamText) {
      const textEl = this._currentStreamEl.querySelector('.stream-text');
      textEl.innerHTML = renderMarkdown(finalMessage.text);
    }

    this._currentStreamEl = null;
    this._currentStreamText = '';
    this._currentToolCalls = {};
    this.state.streaming = false;
    this._updateButtons();
    this._scrollToBottom();
  }

  _finalizeWithError(error) {
    if (!this._currentStreamEl) {
      this._addAssistantMessage(`❌ Error: ${error}`);
    } else {
      const cursor = this._currentStreamEl.querySelector('.streaming-cursor');
      if (cursor) cursor.remove();

      const textEl = this._currentStreamEl.querySelector('.stream-text');
      textEl.innerHTML = renderMarkdown(this._currentStreamText) +
        `<p style="color: var(--error);">❌ ${this._escapeHtml(error)}</p>`;

      this._currentStreamEl = null;
      this._currentStreamText = '';
      this._currentToolCalls = {};
    }
    this.state.streaming = false;
    this._updateButtons();
  }

  _updateButtons() {
    if (this.state.streaming) {
      this.sendBtn.style.display = 'none';
      this.stopBtn.style.display = 'flex';
      this.input.disabled = true;
    } else {
      this.sendBtn.style.display = 'flex';
      this.stopBtn.style.display = 'none';
      this.input.disabled = false;
    }
  }

  _scrollToBottom() {
    requestAnimationFrame(() => {
      this.messageList.scrollTop = this.messageList.scrollHeight;
    });
  }

  _escapeHtml(s) {
    const div = document.createElement('div');
    div.textContent = s;
    return div.innerHTML;
  }
}
