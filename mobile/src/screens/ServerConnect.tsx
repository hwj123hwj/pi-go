/**
 * ServerConnect.tsx — First-run server configuration screen
 *
 * On mobile, the Pi-Go app connects to a remote server. The user must
 * enter the server URL on first launch (stored in SecureStore).
 */

import React, { useState, useEffect } from 'react';
import {
  View, Text, TextInput, TouchableOpacity, StyleSheet,
  ActivityIndicator, KeyboardAvoidingView, Platform,
} from 'react-native';
import { useStore } from '../store';
import { loadStoredServerUrl, setStoredServerUrl, setBaseUrl } from '../api/server-url';

export function ServerConnect({ onConnected }: { onConnected: () => void }) {
  const [url, setUrl] = useState('');
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    (async () => {
      const stored = await loadStoredServerUrl();
      if (stored) setUrl(stored);
    })();
  }, []);

  const handleConnect = async () => {
    setError('');
    let trimmed = url.trim();
    if (!trimmed) {
      setError('请输入服务器地址');
      return;
    }
    // Normalize: ensure protocol
    if (!trimmed.startsWith('http://') && !trimmed.startsWith('https://')) {
      trimmed = 'http://' + trimmed;
    }
    trimmed = trimmed.replace(/\/$/, '');

    setTesting(true);
    try {
      // Test connection
      const res = await fetch(`${trimmed}/health`);
      if (!res.ok) throw new Error(`Server responded ${res.status}`);

      // Save and proceed
      setBaseUrl(trimmed);
      await setStoredServerUrl(trimmed);
      onConnected();
    } catch (err) {
      setError('无法连接服务器: ' + (err instanceof Error ? err.message : '未知错误'));
    } finally {
      setTesting(false);
    }
  };

  return (
    <KeyboardAvoidingView
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}
      style={styles.container}
    >
      <View style={styles.inner}>
        <Text style={styles.logo}>🚀</Text>
        <Text style={styles.title}>Pi-Go</Text>
        <Text style={styles.subtitle}>连接到你的服务器</Text>

        <TextInput
          style={styles.input}
          placeholder="http://192.168.1.100:8080"
          placeholderTextColor="#807d74"
          value={url}
          onChangeText={setUrl}
          autoCapitalize="none"
          autoCorrect={false}
          keyboardType="url"
          onSubmitEditing={handleConnect}
        />

        {error ? <Text style={styles.error}>{error}</Text> : null}

        <TouchableOpacity
          style={styles.button}
          onPress={handleConnect}
          disabled={testing}
        >
          {testing ? (
            <ActivityIndicator color="#fff" />
          ) : (
            <Text style={styles.buttonText}>连接</Text>
          )}
        </TouchableOpacity>
      </View>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#262624' },
  inner: {
    flex: 1, justifyContent: 'center', alignItems: 'center', padding: 32,
  },
  logo: { fontSize: 56, marginBottom: 8 },
  title: { fontSize: 28, fontWeight: '700', color: '#f3f1ea', marginBottom: 4 },
  subtitle: { fontSize: 15, color: '#b3b0a6', marginBottom: 32 },
  input: {
    width: '100%',
    backgroundColor: '#383735',
    color: '#f3f1ea',
    borderRadius: 10,
    padding: 14,
    fontSize: 16,
    marginBottom: 12,
    borderWidth: 1,
    borderColor: '#3a3937',
  },
  button: {
    width: '100%',
    backgroundColor: '#d97757',
    paddingVertical: 14,
    borderRadius: 10,
    alignItems: 'center',
  },
  buttonText: { color: '#fff', fontSize: 16, fontWeight: '600' },
  error: { color: '#e2776e', fontSize: 13, marginBottom: 12, textAlign: 'center' },
});
