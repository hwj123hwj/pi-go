// Sidebar: session list, model selector, connection status

export class Sidebar {
  constructor(ws, state, onSessionChange) {
    this.ws = ws;
    this.state = state;
    this.onSessionChange = onSessionChange;

    this.sessionList = document.getElementById('session-list');
    this.modelSelect = document.getElementById('model-select');
    this.newSessionBtn = document.getElementById('new-session-btn');
    this.connectionStatus = document.getElementById('connection-status');

    this._bindEvents();
    this._bindWS();
  }

  _bindEvents() {
    this.newSessionBtn.addEventListener('click', () => this.createSession());

    this.modelSelect.addEventListener('change', () => {
      const val = this.modelSelect.value;
      if (!val || !this.state.currentSessionId) return;
      const [provider, model] = val.split('/');
      this.ws.sendSwitchModel(this.state.currentSessionId, model, provider);
    });

    // Session list click delegation
    this.sessionList.addEventListener('click', (e) => {
      const item = e.target.closest('.session-item');
      if (!item) return;

      const deleteBtn = e.target.closest('.delete-btn');
      if (deleteBtn) {
        e.stopPropagation();
        this.deleteSession(item.dataset.sessionId);
        return;
      }

      this.selectSession(item.dataset.sessionId);
    });
  }

  _bindWS() {
    this.ws.onStatusChange = (connected) => {
      this._updateConnectionStatus(connected);
    };
  }

  _updateConnectionStatus(connected) {
    const dot = this.connectionStatus.querySelector('.status-dot');
    const text = this.connectionStatus.querySelector('.status-text');

    if (connected) {
      dot.className = 'status-dot online';
      text.textContent = 'Connected';
    } else {
      dot.className = 'status-dot connecting';
      text.textContent = 'Reconnecting...';
    }
  }

  async loadSessions() {
    try {
      const resp = await fetch(`${this.state.baseUrl}/sessions`);
      if (!resp.ok) return;
      this.state.sessions = await resp.json();
      this._renderSessions();
    } catch (e) {
      console.error('Failed to load sessions:', e);
    }
  }

  async loadModels() {
    try {
      const resp = await fetch(`${this.state.baseUrl}/models`);
      if (!resp.ok) return;
      const data = await resp.json();
      this.state.models = data.models || [];
      this._renderModels(data.current);
    } catch (e) {
      console.error('Failed to load models:', e);
    }
  }

  async createSession() {
    try {
      const resp = await fetch(`${this.state.baseUrl}/sessions`, { method: 'POST' });
      if (!resp.ok) return;
      const data = await resp.json();
      this.state.sessions.unshift({
        id: data.id,
        created_at: data.created_at,
        message_count: 0,
        last_active: data.created_at,
      });
      this._renderSessions();
      this.selectSession(data.id);
    } catch (e) {
      console.error('Failed to create session:', e);
    }
  }

  async deleteSession(sessionId) {
    try {
      const resp = await fetch(`${this.state.baseUrl}/sessions/${sessionId}`, { method: 'DELETE' });
      if (!resp.ok) return;
      this.state.sessions = this.state.sessions.filter(s => s.id !== sessionId);
      if (this.state.currentSessionId === sessionId) {
        this.state.currentSessionId = null;
        this.onSessionChange(null);
      }
      this._renderSessions();
    } catch (e) {
      console.error('Failed to delete session:', e);
    }
  }

  selectSession(sessionId) {
    this.state.currentSessionId = sessionId;
    this._renderSessions();
    this.onSessionChange(sessionId);
  }

  _renderSessions() {
    if (this.state.sessions.length === 0) {
      this.sessionList.innerHTML = '<div class="empty-sessions">No sessions yet</div>';
      return;
    }

    this.sessionList.innerHTML = this.state.sessions.map(s => {
      const active = s.id === this.state.currentSessionId ? ' active' : '';
      const title = this._formatSessionTitle(s);
      const meta = this._formatSessionMeta(s);
      return `
        <div class="session-item${active}" data-session-id="${s.id}">
          <div class="session-info">
            <div class="session-title">${title}</div>
            <div class="session-meta">${meta}</div>
          </div>
          <button class="delete-btn" title="Delete">✕</button>
        </div>
      `;
    }).join('');
  }

  _renderModels(current) {
    if (!this.state.models.length) {
      this.modelSelect.innerHTML = '<option value="">No models</option>';
      return;
    }

    this.modelSelect.innerHTML = this.state.models.map(m => {
      const val = `${m.provider}/${m.id}`;
      const selected = current && current.id === m.id ? ' selected' : '';
      return `<option value="${val}"${selected}>${m.name || m.id}</option>`;
    }).join('');
  }

  _formatSessionTitle(s) {
    // Show truncated session ID
    const shortId = s.id.replace('sess_', '').slice(0, 12);
    return shortId;
  }

  _formatSessionMeta(s) {
    const msgCount = s.message_count || 0;
    const time = this._formatTime(s.last_active);
    return `${msgCount} msgs · ${time}`;
  }

  _formatTime(ts) {
    if (!ts) return '';
    const date = new Date(ts * 1000);
    const now = new Date();
    const diff = now - date;

    if (diff < 60000) return 'just now';
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
    return date.toLocaleDateString();
  }
}
