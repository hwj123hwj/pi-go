import { useEffect, useRef } from 'react';
import { useStore, type ChatItem, type SessionView } from '../../store';
import { Markdown } from '../Markdown';
import { ToolCall } from '../ToolCall';
import { MusicPlayer } from '../MusicPlayer';
import { LyricsViewer } from '../LyricsViewer';
import { Icon, toolKindIcon } from '../Icon';
import { useT, type TFunc } from '../../i18n/useT';
import { isElectron } from '../../platform';

export function ChatPane({ view }: { view: SessionView }) {
  const density = view.density;
  const t = useT();
  const bottomRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const stickToBottomRef = useRef(true);

  const busy = view.meta.status === 'thinking' || view.meta.status === 'starting';
  const last = view.transcript[view.transcript.length - 1];
  const showDots = busy && (!last || last.kind !== 'assistant');

  // Track whether the user is scrolled to the bottom of the transcript.
  // If not, don't auto-scroll on new messages (respect their scroll position).
  useEffect(() => {
    const el = scrollContainerRef.current;
    if (!el) return;
    const onScroll = () => {
      const threshold = 80; // px from bottom considered "at the bottom"
      stickToBottomRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < threshold;
    };
    el.addEventListener('scroll', onScroll, { passive: true });
    return () => el.removeEventListener('scroll', onScroll);
  }, []);

  // Auto-scroll to bottom only if the user hasn't scrolled up
  useEffect(() => {
    if (stickToBottomRef.current) {
      bottomRef.current?.scrollIntoView({ behavior: 'auto' });
    }
  }, [view.transcript, showDots]);

  // Listen for file path clicks in Markdown — open in right sidebar Files panel
  useEffect(() => {
    const handleOpenFile = (e: Event) => {
      const customEvent = e as CustomEvent;
      const path = customEvent.detail?.path;
      if (path) {
        useStore.getState().openFileTab(path);
      }
    };
    window.addEventListener('open-file', handleOpenFile);
    return () => window.removeEventListener('open-file', handleOpenFile);
  }, [view.meta.id]);

  return (
    <div
      className="pane-body"
      ref={scrollContainerRef}
      onTouchStart={isElectron ? undefined : (e) => {
        // Dismiss keyboard when touching the transcript scroll area on mobile.
        // Skip if the touch target is a button, link, or inside interactive
        // elements (code copy, tool cards, etc.) — only blur for empty-area scrolls.
        const target = e.target as HTMLElement;
        if (target.closest('button, a, input, textarea, .tool-head, .md-code-copy, .gm-btn')) return;
        const el = document.activeElement;
        if (el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA')) {
          (el as HTMLElement).blur();
        }
      }}
    >
      <div className="transcript">
        {view.transcript.length === 0 && (
          <div className="empty" style={{ height: 280 }}>
            <div className="empty-inner">
              <span className="empty-mark">
                <Icon name="sparkle" size={24} />
              </span>
              <div className="empty-title">{t('chat.emptyTitle')}</div>
            </div>
          </div>
        )}
        {view.transcript.map((item) => (
          <ChatItemView key={item.id} item={item} density={density} t={t} sessionId={view.meta.id} onOpenFile={(path) => useStore.getState().openFileTab(path)} />
        ))}
        {showDots && (
          <div className="msg msg-assistant typing-row">
            <div className="typing-dots" aria-label={t('chat.aiResponding')}>
              <span />
              <span />
              <span />
            </div>
          </div>
        )}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}

function ChatItemView({
  item,
  density,
  t,
  sessionId,
  onOpenFile,
}: {
  item: ChatItem;
  density: SessionView['density'];
  t: TFunc;
  sessionId: string;
  onOpenFile: (path: string) => void;
}) {
  // For assistant messages, we need the cwd as basePath for link resolution.
  const cwd = useStore((s) => s.sessions[sessionId]?.meta.cwd);
  switch (item.kind) {
    case 'user':
      return (
        <div className="msg">
          <div className="msg-user">
            {item.text && <span className="msg-user-text">{item.text}</span>}
          </div>
        </div>
      );

    case 'assistant':
      return (
        <div className="msg msg-assistant">
          <Markdown text={item.text} basePath={cwd} />
        </div>
      );

    case 'thought':
      if (density === 'summary') return null;
      return (
        <div className="msg msg-thought">
          <div className="thought-label">
            <Icon name="think" size={13} />
            {t('chat.thought')}
          </div>
          {item.text}
        </div>
      );

    case 'system':
      if (density === 'summary') return null;
      return <div className="msg-system">{item.text}</div>;

    case 'error':
      return (
        <div className="msg msg-error">
          <Icon name="alert" size={16} />
          <span>{item.text}</span>
        </div>
      );

    case 'tool':
      if (density === 'summary') {
        // Compact single-line summary instead of hiding entirely
        return (
          <div className="tool-summary-line">
            <Icon name={toolKindIcon(item.toolKind)} size={13} />
            <span className="tool-summary-title">{item.title}</span>
            <span className={`tool-summary-status ${item.status}`}>
              {item.status === 'completed' ? '✓' : item.status === 'failed' ? '✗' : '…'}
            </span>
          </div>
        );
      }
      // Music tool special rendering
      if (item.title === 'music_play' && item.status === 'completed') {
        const text = item.content.map((c) => c.text).filter(Boolean).join('\n');
        if (text) return <MusicPlayer resultText={text} details={item.details} />;
      }
      if (item.title === 'music_lyrics' && item.status === 'completed') {
        const text = item.content.map((c) => c.text).filter(Boolean).join('\n');
        if (text) return <LyricsViewer lrcText={text} />;
      }
      return (
        <ToolCall
          title={item.title}
          toolKind={item.toolKind}
          status={item.status}
          locations={item.locations}
          content={item.content}
          terminalOutput={item.terminalOutput}
          rawInput={item.rawInput}
          defaultOpen={density === 'verbose'}
          onOpenFile={onOpenFile}
        />
      );

    default:
      return null;
  }
}
