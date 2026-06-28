/**
 * ChatScreen.tsx — Chat conversation screen (mobile)
 *
 * RN Best Practices applied:
 * - js-lists-flatlist-flashlist: removeClippedSubviews, maxToRenderPerBatch, windowSize
 * - React.memo for message components to prevent cascading re-renders
 * - useCallback for renderItem to keep FlatList stable
 * - js-uncontrolled-components: TextInput with ref for voice append (avoids re-render on programmatic update)
 */

import React, { useState, useRef, useCallback, useEffect, useDeferredValue, memo } from 'react';
import {
  View, Text, TextInput, TouchableOpacity, FlatList, StyleSheet,
  SafeAreaView, ActivityIndicator, Platform,
} from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';

const SafeArea = SafeAreaView as any;
import { useStore } from '../store';
import type { ChatItem } from '../types/ChatItem';
import type { RootStackParamList } from '../navigation/AppNavigator';

type ChatScreenProps = NativeStackScreenProps<RootStackParamList, 'Chat'>;

// ─── Memoized message components (prevents cascading re-renders) ──────

const UserBubble = memo(({ text }: { text: string }) => (
  <View style={msgStyles.userBubble}>
    <Text style={msgStyles.userText}>{text}</Text>
  </View>
));

const AssistantMessage = memo(({ text, busy }: { text: string; busy: boolean }) => (
  <View style={msgStyles.assistantRow}>
    {text === '' && busy ? (
      <ActivityIndicator color="#d97757" size="small" />
    ) : (
      <Text style={msgStyles.assistantText}>{text}</Text>
    )}
  </View>
));

const ToolMessage = memo(({ title, status, text }: { title: string; status?: string; text?: string }) => (
  <View style={msgStyles.toolRow}>
    <Text style={msgStyles.toolTitle}>
      {status === 'failed' ? '❌' : '✅'} {title}
    </Text>
    {text ? (
      <Text style={msgStyles.toolResult} numberOfLines={8}>{text}</Text>
    ) : null}
  </View>
));

const ThoughtMessage = memo(({ text }: { text: string }) => (
  <View style={msgStyles.thoughtRow}>
    <Text style={msgStyles.thoughtText}>💭 {text}</Text>
  </View>
));

const ErrorMessage = memo(({ text }: { text: string }) => (
  <View style={msgStyles.errorRow}>
    <Text style={msgStyles.errorText}>⚠️ {text}</Text>
  </View>
));

// ─── Chat Screen ──────────────────────────────────────────────────────

export function ChatScreen({ route, navigation }: ChatScreenProps) {
  const { sessionId } = route.params;
  const onBack = navigation.goBack;
  const view = useStore(useCallback((s) => s.sessions[sessionId], [sessionId]));
  const sendPrompt = useStore(useCallback((s) => s.sendPrompt, []));
  const cancel = useStore(useCallback((s) => s.cancel, []));
  const models = useStore(useCallback((s) => s.models, []));
  const setModel = useStore(useCallback((s) => s.setModel, []));

  const [text, setText] = useState('');
  const flatRef = useRef<FlatList>(null);
  // ── js-uncontrolled-components: ref for TextInput to read value without re-render ──
  const inputRef = useRef<TextInput>(null);
  const textRef = useRef('');

  const busy = view?.meta.status === 'thinking' || view?.meta.status === 'starting';
  const transcript = view?.transcript ?? [];

  // ── js-concurrent-react: Defer transcript rendering during streaming ──
  // The raw transcript updates on every text_delta (high frequency).
  // deferredTranscript lets React prioritize input/scroll over list re-render.
  const deferredTranscript = useDeferredValue(transcript);
  const deferredBusy = useDeferredValue(busy);
  const transcriptLen = deferredTranscript.length;

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    if (transcriptLen > 0) {
      setTimeout(() => flatRef.current?.scrollToEnd({ animated: true }), 50);
    }
  }, [transcriptLen]);

  const submit = useCallback(async () => {
    const trimmed = textRef.current.trim();
    if (!trimmed || busy) return;
    textRef.current = '';
    setText('');
    inputRef.current?.setNativeProps({ text: '' });
    await sendPrompt(sessionId, trimmed);
  }, [busy, sessionId, sendPrompt]);

  // ── renderItem uses deferredBusy/deferredTranscript for non-blocking render ──

  // ─── renderItem: stable callback with React.memo children ──────────
  const renderItem = useCallback(({ item }: { item: ChatItem }) => {
    switch (item.kind) {
      case 'user':
        return <UserBubble text={item.text} />;
      case 'assistant':
        return <AssistantMessage text={item.text} busy={deferredBusy} />;
      case 'tool':
        return <ToolMessage title={item.title || ''} status={item.status} text={item.text} />;
      case 'thought':
        return <ThoughtMessage text={item.text} />;
      case 'error':
        return <ErrorMessage text={item.text} />;
      default:
        return null;
    }
  }, [deferredBusy]);

  const keyExtractor = useCallback((item: ChatItem) => item.id, []);

  const getItemLayout = useCallback(
    (_: any, index: number) => ({
      length: 60, // estimated item height
      offset: 60 * index,
      index,
    }),
    [],
  );

  const currentModel = view?.meta.model ?? '';

  return (
    <SafeArea style={styles.container} edges={['bottom']} collapsable={false}>
      {/* Header */}
      <View style={styles.header}>
        <TouchableOpacity onPress={onBack} style={styles.backBtn}>
          <Text style={styles.backText}>←</Text>
        </TouchableOpacity>
        <Text style={styles.headerTitle} numberOfLines={1}>
          {view?.meta.title ?? 'Chat'}
        </Text>
        {/* Model selector */}
        {models.length > 0 && (
          <View style={styles.modelPicker}>
            {models.map((m) => (
              <TouchableOpacity
                key={m.modelId}
                style={[
                  styles.modelChip,
                  (currentModel === m.modelId || (!currentModel && models.indexOf(m) === 0)) &&
                    styles.modelChipActive,
                ]}
                onPress={() => setModel(sessionId, m.modelId)}
              >
                <Text
                  style={[
                    styles.modelChipText,
                    (currentModel === m.modelId || (!currentModel && models.indexOf(m) === 0)) &&
                      styles.modelChipTextActive,
                  ]}
                >
                  {m.name}
                </Text>
              </TouchableOpacity>
            ))}
          </View>
        )}
      </View>

      {/* Messages — optimized FlatList */}
      {/* ── native-view-flattening: prevent parent view flattening issues ── */}
      <FlatList
        ref={flatRef}
        data={deferredTranscript}
        keyExtractor={keyExtractor}
        renderItem={renderItem}
        getItemLayout={getItemLayout}
        removeClippedSubviews={true}
        maxToRenderPerBatch={8}
        windowSize={12}
        initialNumToRender={12}
        contentContainerStyle={{ padding: 12, paddingBottom: 8 }}
        onContentSizeChange={() => flatRef.current?.scrollToEnd({ animated: false })}
      />

      {/* Input bar */}
      <View style={styles.inputBar}>
        {/* ── js-uncontrolled-components: uncontrolled TextInput via ref ── */}
        <TextInput
          ref={inputRef}
          style={styles.input}
          placeholder="输入消息..."
          placeholderTextColor="#807d74"
          defaultValue=""
          onChangeText={(t) => { textRef.current = t; setText(t); }}
          multiline
          maxLength={8000}
        />

        {/* Send / Stop button */}
        {busy ? (
          <TouchableOpacity style={styles.stopBtn} onPress={() => cancel(sessionId)}>
            <Text style={styles.stopBtnText}>⏹</Text>
          </TouchableOpacity>
        ) : (
          <TouchableOpacity
            style={[styles.sendBtn, !text.trim() && styles.sendBtnDisabled]}
            onPress={submit}
            disabled={!text.trim()}
          >
            <Text style={styles.sendBtnText}>↑</Text>
          </TouchableOpacity>
        )}
      </View>
    </SafeArea>
  );
}

// ─── Styles ────────────────────────────────────────────────────────────

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#262624' },
  header: {
    flexDirection: 'row', alignItems: 'center', gap: 8,
    paddingHorizontal: 12, paddingVertical: 10,
    borderBottomWidth: 1, borderBottomColor: '#3a3937',
  },
  backBtn: { width: 32, height: 32, justifyContent: 'center' },
  backText: { fontSize: 22, color: '#d97757' },
  headerTitle: { fontSize: 16, fontWeight: '600', color: '#f3f1ea', flex: 1 },
  modelPicker: { flexDirection: 'row', gap: 4 },
  modelChip: {
    paddingHorizontal: 8, paddingVertical: 4, borderRadius: 6,
    backgroundColor: '#383735',
  },
  modelChipActive: { backgroundColor: 'rgba(217,119,87,0.2)' },
  modelChipText: { fontSize: 11, color: '#b3b0a6' },
  modelChipTextActive: { color: '#d97757' },
  inputBar: {
    flexDirection: 'row', alignItems: 'flex-end', gap: 8,
    paddingHorizontal: 12, paddingVertical: 8,
    borderTopWidth: 1, borderTopColor: '#3a3937',
    paddingBottom: Platform.OS === 'ios' ? 8 : 12,
  },
  input: {
    flex: 1, backgroundColor: '#383735', color: '#f3f1ea',
    borderRadius: 10, paddingHorizontal: 14, paddingVertical: 10,
    fontSize: 15, maxHeight: 100, minHeight: 40,
    borderWidth: 1, borderColor: '#3a3937',
  },
  sendBtn: {
    width: 40, height: 40, borderRadius: 20,
    backgroundColor: '#d97757', justifyContent: 'center', alignItems: 'center',
  },
  sendBtnDisabled: { opacity: 0.3 },
  sendBtnText: { color: '#fff', fontSize: 20, fontWeight: '700' },
  stopBtn: {
    width: 40, height: 40, borderRadius: 20,
    backgroundColor: 'rgba(226,119,110,0.2)', justifyContent: 'center', alignItems: 'center',
    borderWidth: 1, borderColor: '#e2776e',
  },
  stopBtnText: { color: '#e2776e', fontSize: 16 },
});

const msgStyles = StyleSheet.create({
  userBubble: {
    alignSelf: 'flex-end', backgroundColor: '#d97757',
    borderRadius: 14, padding: 12, maxWidth: '85%',
    marginBottom: 8,
  },
  userText: { color: '#fff', fontSize: 15, lineHeight: 22 },
  assistantRow: {
    alignSelf: 'flex-start', maxWidth: '90%',
    marginBottom: 8, padding: 4,
  },
  assistantText: { color: '#f3f1ea', fontSize: 15, lineHeight: 22 },
  toolRow: {
    alignSelf: 'flex-start', maxWidth: '90%',
    backgroundColor: '#2f2e2c', borderRadius: 8, padding: 10, marginBottom: 6,
    borderLeftWidth: 3, borderLeftColor: '#6cc28a',
  },
  toolTitle: { color: '#b3b0a6', fontSize: 13, fontWeight: '600', marginBottom: 4 },
  toolResult: { color: '#807d74', fontSize: 12, lineHeight: 18 },
  thoughtRow: {
    alignSelf: 'flex-start', maxWidth: '90%',
    marginBottom: 6,
  },
  thoughtText: { color: '#807d74', fontSize: 13, fontStyle: 'italic' },
  errorRow: {
    alignSelf: 'flex-start', maxWidth: '90%',
    backgroundColor: 'rgba(226,119,110,0.12)', borderRadius: 8,
    padding: 10, marginBottom: 6,
  },
  errorText: { color: '#e2776e', fontSize: 13 },
});
