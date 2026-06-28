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
  const [initialRoute, setInitialRoute] = useState<'Connect' | 'List'>('Connect');

  // ── BUGFIX: Track init promise + catch rejections to avoid unhandled rejection ──
  const initPromiseRef = useRef<Promise<void> | null>(null);
  // ── BUGFIX: Key to force navigator remount on ErrorBoundary reset ──
  const [navKey, setNavKey] = useState(0);

  useEffect(() => {
    if (!initPromiseRef.current) {
      initPromiseRef.current = (async () => {
        try {
          // ── BUGFIX B: init() now has internal try/catch, but guard anyway ──
          const ok = await init();
          if (ok) setInitialRoute('List');
        } catch (err) {
          console.error('App init failed:', err);
          setInitialRoute('Connect');
        }
      })();
    }
    // ── BUGFIX A: Do NOT call destroy() in root component cleanup.
    // The root <App> should never unmount. In React Strict Mode (dev),
    // effects mount→unmount→mount, so destroy() would kill WS and
    // initPromiseRef would prevent re-init on re-mount.
    // Instead, destroy() is for explicit user-initiated disconnect only.
  }, []);

  // ── BUGFIX D: ErrorBoundary reset now increments navKey to remount navigator ──
  const handleErrorReset = () => {
    setNavKey((k) => k + 1);
    setInitialRoute('Connect');
  };

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
      <ErrorBoundary onReset={handleErrorReset}>
        <AppNavigator key={navKey} initialRoute={initialRoute} />
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
