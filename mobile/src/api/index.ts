/**
 * api.ts — REST + WebSocket helpers for Pi-Go mobile
 *
 * All HTTP requests go through this module. Base URL is configured at runtime
 * from SecureStore (set by the ServerConnect screen).
 */

import * as SecureStore from 'expo-secure-store';

let baseUrl = '';

const SERVER_URL_KEY = 'pigo_server_url';

export async function loadStoredServerUrl(): Promise<string | null> {
  return SecureStore.getItemAsync(SERVER_URL_KEY);
}

export async function setStoredServerUrl(url: string): Promise<void> {
  await SecureStore.setItemAsync(SERVER_URL_KEY, url);
}

export function setBaseUrl(url: string): void {
  baseUrl = url.replace(/\/$/, '');
}

export function getBaseUrl(): string {
  return baseUrl;
}

export async function apiRequest<T>(
  method: string,
  path: string,
  body?: Record<string, unknown>,
): Promise<T> {
  const res = await fetch(`${baseUrl}${path}`, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    const err = await res.text().catch(() => 'unknown');
    throw new Error(`${res.status}: ${err}`);
  }
  return res.json() as Promise<T>;
}

/**
 * Upload a file (audio blob) for ASR transcription.
 */
export async function uploadForASR(uri: string, mimeType: string): Promise<{ text: string }> {
  const formData = new FormData();
  formData.append('file', {
    uri,
    type: mimeType,
    name: 'voice.m4a',
  } as unknown as Blob);

  const res = await fetch(`${baseUrl}/asr/transcribe`, {
    method: 'POST',
    body: formData,
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'ASR request failed' }));
    throw new Error(err.error || `Server error ${res.status}`);
  }

  return res.json();
}
