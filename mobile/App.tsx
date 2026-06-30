/**
 * App.tsx — Root component for Pi-Go mobile (React Native / Expo)
 *
 * RN Best Practices applied:
 * - js-atomic-state: Fine-grained Zustand selectors (no broad re-renders)
 * - ErrorBoundary wrapping navigation to prevent white-screen crashes
 * - react-native-screens: Native navigation stack (native-screen-stack)
 */

import React, { useState, useEffect, useRef } from 'react';
import { StatusBar } from 'expo-status-bar';
import { GestureHandlerRootView } from 'react-native-gesture-handler';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { useStore } from './src/store';
import { ErrorBoundary } from './src/components/ErrorBoundary';
import { AppNavigator } from './src/navigation/AppNavigator';

export default function App() {
  const init = useStore((s) => s.init);
  const ready = useStore((s) => s.ready);
  const serverReady = useStore((s) => s.serverReady);
  const [initialRoute, setInitialRoute] = useState<'Connect' | 'List'>('Connect');

  const initPromiseRef = useRef<Promise<void> | null>(null);
  const [navKey, setNavKey] = useState(0);

  useEffect(() => {
    if (!initPromiseRef.current) {
      initPromiseRef.current = (async () => {
        try {
          const ok = await init();
          if (ok) setInitialRoute('List');
        } catch (err) {
          console.error('App init failed:', err);
          setInitialRoute('Connect');
        }
      })();
    }
  }, []);

  const handleErrorReset = () => {
    setNavKey((k) => k + 1);
    setInitialRoute('Connect');
  };

  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      <SafeAreaProvider>
        <StatusBar style="light" />
        <ErrorBoundary onReset={handleErrorReset}>
          <AppNavigator key={navKey} initialRoute={initialRoute} />
        </ErrorBoundary>
      </SafeAreaProvider>
    </GestureHandlerRootView>
  );
}
