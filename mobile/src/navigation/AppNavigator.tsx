/**
 * AppNavigator.tsx — Native navigation stack for Pi-Go mobile
 *
 * RN Best Practice: native-sdks-over-polyfills + TTI optimization
 * - react-native-screens: Native screen containers (not JS-managed views)
 * - @react-navigation/native-stack: Hardware back button, native transitions
 * - Screens are kept in native stack → lower memory, faster transitions
 */

import React from 'react';
import { NavigationContainer, DarkTheme } from '@react-navigation/native';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { ServerConnect } from '../screens/ServerConnect';
import { SessionList } from '../screens/SessionList';
import { ChatScreen } from '../screens/ChatScreen';

export type RootStackParamList = {
  Connect: undefined;
  List: undefined;
  Chat: { sessionId: string };
};

const Stack = createNativeStackNavigator<RootStackParamList>();

const navTheme = {
  ...DarkTheme,
  colors: {
    ...DarkTheme.colors,
    background: '#262624',
    card: '#2f2e2c',
    text: '#f3f1ea',
    primary: '#d97757',
    border: '#3a3937',
  },
};

export function AppNavigator({ initialRoute }: { initialRoute: keyof RootStackParamList }) {
  return (
    <NavigationContainer theme={navTheme}>
      <Stack.Navigator
        initialRouteName={initialRoute}
        screenOptions={{
          headerShown: false,
          contentStyle: { backgroundColor: '#262624' },
          // ── native-screen optimization: freeze off-screen screens ──
          freezeOnBlur: true,
          // Native animation for screen transitions
          animation: 'slide_from_right',
        }}
      >
        <Stack.Screen name="Connect" component={ServerConnect} />
        <Stack.Screen name="List" component={SessionList} />
        <Stack.Screen name="Chat" component={ChatScreen} />
      </Stack.Navigator>
    </NavigationContainer>
  );
}
