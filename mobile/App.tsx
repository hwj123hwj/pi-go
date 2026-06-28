/**
 * App.tsx — Root component for Pi-Go mobile (React Native / Expo)
 *
 * RN Best Practices applied:
 * - js-atomic-state: Fine-grained Zustand selectors (no broad re-renders)
 * - ErrorBoundary wrapping navigation to prevent white-screen crashes
 * - react-native-screens: Native navigation stack (native-screen-stack)
 * - React Compiler: Automatic memoization (babel-plugin-react-compiler)
 */

import React, { useState, useEffect, useRef } from 'react';
import { StatusBar } from 'expo-status-bar';
import { View, Text, ActivityIndicator, StyleSheet } from 'react-native';
import { useStore } from './src/store';
import { ErrorBoundary } from './src/components/ErrorBoundary';
import { AppNavigator } from './src/navigation/AppNavigator';

export default function App() {
  const init = useStore((s) => s.init);
  const ready = useStore((s) => s.ready);
  const serverReady = useStore((s) => s.serverReady);
  const destroy = useStore((s) => s.destroy);
  const [initialRoute, setInitialRoute] = useState<'Connect' | 'List'>('Connect');

  // ── BUGFIX: Track init promise to avoid double-init race ──
  const initPromiseRef = useRef<Promise<void> | null>(null);

  useEffect(() => {
    if (!initPromiseRef.current) {
      initPromiseRef.current = (async () => {
        const ok = await init();
        if (ok) setInitialRoute('List');
      })();
    }
    return () => {
      // ── BUGFIX: Clean up WS listeners on unmount (js-memory-leaks) ──
      destroy();
    };
  }, []);

  if (!ready && serverReady) {
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

  return (
    <>
      <StatusBar style="light" />
      <ErrorBoundary onReset={() => setInitialRoute('Connect')}>
        <AppNavigator initialRoute={initialRoute} />
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
