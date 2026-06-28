/**
 * App.tsx — Root component for Pi-Go mobile (React Native / Expo)
 *
 * RN Best Practices applied:
 * - js-atomic-state: Fine-grained Zustand selectors (no broad re-renders)
 * - useCallback for event handlers to keep child renders stable
 * - ErrorBoundary wrapping each screen to prevent white-screen crashes
 *
 * Screen flow:
 *   ServerConnect → SessionList → ChatScreen
 */

import React, { useState, useEffect, useCallback } from 'react';
import { StatusBar } from 'expo-status-bar';
import { View, Text, ActivityIndicator, StyleSheet } from 'react-native';
import { useStore } from './src/store';
import { ErrorBoundary } from './src/components/ErrorBoundary';
import { ServerConnect } from './src/screens/ServerConnect';
import { SessionList } from './src/screens/SessionList';
import { ChatScreen } from './src/screens/ChatScreen';

type Screen = 'connect' | 'list' | 'chat';

export default function App() {
  const init = useStore((s) => s.init);
  const ready = useStore((s) => s.ready);
  const serverReady = useStore((s) => s.serverReady);
  const [screen, setScreen] = useState<Screen>('connect');
  const [chatSessionId, setChatSessionId] = useState<string | null>(null);

  useEffect(() => {
    (async () => {
      await init();
      const { serverReady: sr } = useStore.getState();
      if (sr) setScreen('list');
    })();
  }, []);

  const handleConnected = useCallback(() => {
    void init();
    setScreen('list');
  }, [init]);

  const handleOpenSession = useCallback((id: string) => {
    setChatSessionId(id);
    setScreen('chat');
  }, []);

  const handleBack = useCallback(() => {
    setChatSessionId(null);
    setScreen('list');
  }, []);

  if (screen === 'connect' && !serverReady) {
    return (
      <>
        <StatusBar style="light" />
        <ErrorBoundary onReset={() => setScreen('connect')}>
          <ServerConnect onConnected={handleConnected} />
        </ErrorBoundary>
      </>
    );
  }

  if (!ready) {
    return (
      <>
        <StatusBar style="light" />
        <View style={bootStyles.container}>
          <Text style={bootStyles.logo}>🚀</Text>
          <ActivityIndicator color="#d97757" size="large" />
        </View>
      </>
    );
  }

  if (screen === 'chat' && chatSessionId) {
    return (
      <>
        <StatusBar style="light" />
        <ErrorBoundary onReset={handleBack}>
          <ChatScreen sessionId={chatSessionId} onBack={handleBack} />
        </ErrorBoundary>
      </>
    );
  }

  return (
    <>
      <StatusBar style="light" />
      <ErrorBoundary onReset={() => setScreen('connect')}>
        <SessionList onOpenSession={handleOpenSession} />
      </ErrorBoundary>
    </>
  );
}

const bootStyles = StyleSheet.create({
  container: {
    flex: 1, backgroundColor: '#262624',
    justifyContent: 'center', alignItems: 'center', gap: 16,
  },
  logo: { fontSize: 56 },
});
