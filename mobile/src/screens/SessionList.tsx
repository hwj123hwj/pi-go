/**
 * SessionList.tsx — Chat session list screen (mobile home)
 *
 * RN Best Practices applied:
 * - js-lists-flatlist-flashlist: removeClippedSubviews, maxToRenderPerBatch, windowSize
 * - React.memo for SessionCard to prevent cascading re-renders
 * - useCallback for renderItem and keyExtractor
 */

import React, { useState, useCallback, memo } from 'react';
import {
  View, Text, FlatList, TouchableOpacity, StyleSheet,
  RefreshControl, SafeAreaView, Modal, Pressable,
} from 'react-native';

const SafeArea = SafeAreaView as any;
import { useStore } from '../store';
import type { SessionView } from '../types/SessionView';

// ─── Memoized card component ──────────────────────────────────────────

const SessionCard = memo(({ view, onPress }: { view: SessionView; onPress: (id: string) => void }) => {
  const statusCol = statusColor(view.meta.status);
  return (
    <TouchableOpacity
      style={styles.card}
      onPress={() => onPress(view.meta.id)}
      activeOpacity={0.7}
    >
      <View style={styles.cardTop}>
        <View style={[styles.dot, { backgroundColor: statusCol }]} />
        <Text style={styles.title} numberOfLines={1}>{view.meta.title}</Text>
      </View>
      {view.meta.cwd ? (
        <Text style={styles.subtitle} numberOfLines={1}>{view.meta.cwd}</Text>
      ) : null}
    </TouchableOpacity>
  );
});

// ─── Screen ───────────────────────────────────────────────────────────

export function SessionList({ onOpenSession }: { onOpenSession: (id: string) => void }) {
  const sessions = useStore(useCallback((s) => s.sessions, []));
  const order = useStore(useCallback((s) => s.order, []));
  const createSession = useStore(useCallback((s) => s.createSession, []));
  const refreshSessions = useStore(useCallback((s) => s.refreshSessions, []));
  const [refreshing, setRefreshing] = useState(false);
  const [showNewMenu, setShowNewMenu] = useState(false);

  const views = order.map((id) => sessions[id]).filter(Boolean) as SessionView[];

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await refreshSessions();
    setRefreshing(false);
  }, [refreshSessions]);

  const handleNew = useCallback(async () => {
    setShowNewMenu(false);
    const id = await createSession();
    onOpenSession(id);
  }, [createSession, onOpenSession]);

  const renderItem = useCallback(({ item }: { item: SessionView }) => (
    <SessionCard view={item} onPress={onOpenSession} />
  ), [onOpenSession]);

  const keyExtractor = useCallback((item: SessionView) => item.meta.id, []);

  return (
    <SafeArea style={styles.container} edges={['top']}>
      <View style={styles.header}>
        <Text style={styles.headerTitle}>Pi-Go</Text>
        <TouchableOpacity onPress={() => setShowNewMenu(true)} style={styles.newBtn}>
          <Text style={styles.newBtnText}>＋</Text>
        </TouchableOpacity>
      </View>

      <FlatList
        data={views}
        keyExtractor={keyExtractor}
        renderItem={renderItem}
        removeClippedSubviews={true}
        maxToRenderPerBatch={6}
        windowSize={10}
        initialNumToRender={10}
        contentContainerStyle={{ padding: 12, paddingBottom: 100 }}
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor="#d97757" />
        }
        ListEmptyComponent={
          <View style={styles.empty}>
            <Text style={styles.emptyText}>还没有会话{'\n'}点击 ＋ 创建</Text>
          </View>
        }
      />

      {/* New Session Menu */}
      <Modal visible={showNewMenu} transparent animationType="fade">
        <Pressable style={styles.overlay} onPress={() => setShowNewMenu(false)}>
          <View style={styles.menu}>
            <TouchableOpacity style={styles.menuItem} onPress={handleNew}>
              <Text style={styles.menuItemText}>💬 新建对话</Text>
            </TouchableOpacity>
          </View>
        </Pressable>
      </Modal>
    </SafeArea>
  );
}

function statusColor(status: string): string {
  switch (status) {
    case 'thinking': return '#d6a94a';
    case 'error': return '#e2776e';
    default: return '#6cc28a';
  }
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#262624' },
  header: {
    flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between',
    paddingHorizontal: 16, paddingVertical: 12,
    borderBottomWidth: 1, borderBottomColor: '#3a3937',
  },
  headerTitle: { fontSize: 20, fontWeight: '700', color: '#f3f1ea' },
  newBtn: {
    width: 36, height: 36, borderRadius: 18,
    backgroundColor: '#d97757', justifyContent: 'center', alignItems: 'center',
  },
  newBtnText: { color: '#fff', fontSize: 22, fontWeight: '300', marginTop: -2 },
  card: {
    backgroundColor: '#2f2e2c', borderRadius: 12, padding: 14, marginBottom: 8,
    borderWidth: 1, borderColor: '#3a3937',
  },
  cardTop: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  dot: { width: 8, height: 8, borderRadius: 4 },
  title: { fontSize: 15, fontWeight: '500', color: '#f3f1ea', flex: 1 },
  subtitle: { fontSize: 12, color: '#807d74', marginTop: 4, marginLeft: 16 },
  empty: { flex: 1, justifyContent: 'center', alignItems: 'center', paddingTop: 80 },
  emptyText: { color: '#807d74', fontSize: 15, textAlign: 'center', lineHeight: 24 },
  overlay: { flex: 1, backgroundColor: 'rgba(0,0,0,0.4)', justifyContent: 'flex-end' },
  menu: {
    backgroundColor: '#2f2e2c', borderRadius: 16, padding: 8,
    marginBottom: 40, marginHorizontal: 40,
  },
  menuItem: { padding: 16, borderRadius: 10 },
  menuItemText: { color: '#f3f1ea', fontSize: 16 },
});
