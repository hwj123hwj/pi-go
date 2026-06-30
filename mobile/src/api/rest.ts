/**
 * rest.ts — REST API request helper for Pi-Go mobile
 *
 * Split from api/index.ts to avoid barrel exports (bundle-barrel-exports).
 */

import { getBaseUrl } from './server-url';

/** fetch with timeout — prevents hangs on slow/blocked networks */
async function fetchWithTimeout(
  url: string,
  options: RequestInit = {},
  timeoutMs = 15_000,
): Promise<Response> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetch(url, { ...options, signal: controller.signal });
    return res;
  } finally {
    clearTimeout(timeoutId);
  }
}

export async function apiRequest<T>(
  method: string,
  path: string,
  body?: Record<string, unknown>,
): Promise<T> {
  const res = await fetchWithTimeout(`${getBaseUrl()}${path}`, {
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

  const res = await fetchWithTimeout(
    `${getBaseUrl()}/asr/transcribe`,
    { method: 'POST', body: formData },
    30_000, // longer timeout for audio upload
  );

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'ASR request failed' }));
    throw new Error(err.error || `Server error ${res.status}`);
  }

  return res.json();
}
