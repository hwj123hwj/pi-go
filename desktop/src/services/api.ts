// api.ts — REST API wrapper for pi-go server.

let baseUrl = 'http://127.0.0.1:8080';

export function setBaseUrl(url: string): void {
  baseUrl = url;
}

export function getBaseUrl(): string {
  return baseUrl;
}

async function request<T>(method: string, path: string, body?: any): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
  };
  if (body !== undefined) {
    opts.body = JSON.stringify(body);
  }

  const res = await fetch(`${baseUrl}${path}`, opts);
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `HTTP ${res.status}`);
  }
  return res.json();
}

// Health
export function health(): Promise<{ status: string }> {
  return request('GET', '/health');
}

// Sessions
export interface SessionInfo {
  id: string;
  message_count?: number;
  last_active?: number;
  created_at?: number;
}

export function listSessions(): Promise<SessionInfo[]> {
  return request('GET', '/sessions');
}

export function createSession(): Promise<{ id: string; created_at: number }> {
  return request('POST', '/sessions');
}

export function getSessionMessages(sessionId: string): Promise<any[]> {
  return request('GET', `/sessions/${sessionId}/messages`);
}

export function getSessionInfo(sessionId: string): Promise<any> {
  return request('GET', `/sessions/${sessionId}/info`);
}

export function deleteSession(sessionId: string): Promise<{ status: string }> {
  return request('DELETE', `/sessions/${sessionId}`);
}

// Models
export interface ModelInfo {
  id: string;
  provider: string;
  name: string;
}

export interface ModelsResponse {
  models: ModelInfo[];
  current?: ModelInfo;
}

export function listModels(): Promise<ModelsResponse> {
  return request('GET', '/models');
}

export function switchModel(sessionId: string, model: string): Promise<{ provider: string; model: string }> {
  return request('POST', `/sessions/${sessionId}/model`, { model });
}

// Tools
export function listTools(): Promise<{ tools: string[] }> {
  return request('GET', '/tools');
}
