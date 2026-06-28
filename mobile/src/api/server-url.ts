/**
 * server-url.ts — SecureStore helpers for server URL persistence
 *
 * Split from api/index.ts to avoid barrel exports (bundle-barrel-exports).
 */

import * as SecureStore from 'expo-secure-store';

const SERVER_URL_KEY = 'pigo_server_url';

let baseUrl = '';

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
