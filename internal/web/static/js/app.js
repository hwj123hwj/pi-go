// Main application — initializes all modules

import { PiWebSocket } from './websocket.js';
import { ChatPanel } from './chat.js';
import { Sidebar } from './sidebar.js';

// Determine base URL (same host serving this page)
const baseUrl = window.location.protocol + '//' + window.location.host;

// Global state
const state = {
  baseUrl,
  currentSessionId: null,
  sessions: [],
  models: [],
  streaming: false,
};

// Initialize modules
const ws = new PiWebSocket(baseUrl);
const chat = new ChatPanel(ws, state);
const sidebar = new Sidebar(ws, state, onSessionChange);

// Connect WebSocket
ws.connect();

// Load initial data
sidebar.loadSessions();
sidebar.loadModels();

// Handle session change
function onSessionChange(sessionId) {
  if (sessionId) {
    chat.show();
    chat.clear();
    chat.loadHistory(sessionId);
  } else {
    chat.hide();
  }
}

// Periodic ping to keep connection alive
setInterval(() => {
  if (ws.connected) ws.sendPing();
}, 30000);
