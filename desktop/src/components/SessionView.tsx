import { useEffect, useRef, useState } from 'react';
import { useStore, type SessionView as SV, type ViewDensity } from '../store';
import { ChatPane } from './panes/ChatPane';
import { PromptBar } from './PromptBar';
import { Icon } from './Icon';
import { WorkspaceToggles } from './workspace/WorkspaceToggles';
import { useT, type TFunc } from '../i18n/useT';

export function SessionView() {
  const activeId = useStore((s) => s.activeSessionId);
  const view = useStore((s) => (activeId ? s.sessions[activeId] : undefined));
  const setDensity = useStore((s) => s.setDensity);
  const t = useT();

  if (!view) {
    return <EmptyState />;
  }

  const meta = view.meta;

  return (
    <main className="main">
      <div className="toolbar">
        <span className="toolbar-title">{meta.title}</span>
        {meta.cwd && (
          <span className="toolbar-cwd" title={meta.cwd}>
            <Icon name="folder" size={13} />
            {baseName(meta.cwd)}
          </span>
        )}
        <span className="toolbar-status">
          <span className={`status-dot ${meta.status}`} />
          {t(`status.${meta.status}` as any)}
        </span>
        <span className="grow" />

        <WorkspaceToggles />

        {/* Density toggle (summary / normal / verbose) */}
        <div className="views-menu toolbar-density">
          {(['summary', 'normal', 'verbose'] as ViewDensity[]).map((d) => (
            <button
              key={d}
              className={view.density === d ? 'active' : ''}
              onClick={() => setDensity(meta.id, d)}
              title={t('density.title')}
            >
              {t(`density.${d}`)}
            </button>
          ))}
        </div>
      </div>

      <div className="workspace">
        <div className="pane">
          <div className="pane-head">
            <Icon name="chat" size={15} />
            <span>{t('pane.chat')}</span>
            <span className="grow" />
          </div>
          <ChatPane view={view} />
        </div>
      </div>

      <PromptBar view={view} />
    </main>
  );
}

function baseName(cwd: string): string {
  const parts = cwd.replace(/[\\/]+$/, '').split(/[\\/]/);
  return parts[parts.length - 1] || cwd;
}

function EmptyState() {
  const createSession = useStore((s) => s.createSession);
  const sendPrompt = useStore((s) => s.sendPrompt);
  const pickFolder = useStore((s) => s.pickFolder);
  const models = useStore((s) => s.models);
  const currentModel = useStore((s) => s.currentModel);
  const t = useT();

  const [text, setText] = useState('');
  const [busy, setBusy] = useState(false);
  const [cwd, setCwd] = useState<string | null>(null);
  const [model, setModel] = useState('');
  const [menuOpen, setMenuOpen] = useState(false);
  const taRef = useRef<HTMLTextAreaElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    taRef.current?.focus();
  }, []);

  useEffect(() => {
    if (currentModel && !model) setModel(currentModel);
  }, [currentModel, model]);

  // Dismiss the project menu on outside click / Escape.
  useEffect(() => {
    if (!menuOpen) return;
    const onDown = (e: MouseEvent) => {
      if (!menuRef.current?.contains(e.target as Node)) setMenuOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setMenuOpen(false);
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [menuOpen]);

  const handlePickFolder = async () => {
    const folder = await pickFolder();
    if (folder) setCwd(folder);
    setMenuOpen(false);
  };

  const submit = async () => {
    const trimmed = text.trim();
    if (!trimmed || busy) return;
    setBusy(true);
    try {
      const id = await createSession({
        cwd: cwd || undefined,
        model: model || undefined,
      });
      await sendPrompt(id, trimmed);
      setText('');
    } catch {
      setBusy(false);
    }
  };

  const targetLabel = cwd ? baseName(cwd) : t('session.emptyChatTarget');

  return (
    <main className="main">
      <div className="empty-titlebar" />
      <div className="empty">
        <div className="empty-inner">
          <div className="empty-title">{t('session.emptyPrompt')}</div>
          <div className="empty-card">
            <textarea
              ref={taRef}
              className="empty-input"
              rows={2}
              placeholder={t('session.emptyPlaceholder')}
              value={text}
              disabled={busy}
              onChange={(e) => setText(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  void submit();
                }
              }}
            />
            <div className="empty-controls">
              {/* Project / directory selector */}
              <div className="empty-target" ref={menuRef}>
                <button
                  className="chip interactive"
                  onClick={() => setMenuOpen((o) => !o)}
                  title={cwd ?? t('session.emptyChatTarget')}
                >
                  <Icon name="folder" size={14} />
                  {targetLabel}
                  <Icon name="chevron-down" size={12} />
                </button>
                {menuOpen && (
                  <div className="empty-menu">
                    <button
                      className={`empty-menu-item ${!cwd ? 'active' : ''}`}
                      onClick={() => {
                        setCwd(null);
                        setMenuOpen(false);
                      }}
                    >
                      <Icon name="sparkle" size={14} />
                      {t('session.emptyChatTarget')}
                    </button>
                    <div className="empty-menu-sep" />
                    <button className="empty-menu-item" onClick={() => void handlePickFolder()}>
                      <Icon name="folder-open" size={14} />
                      {t('session.emptyPickFolder')}
                    </button>
                  </div>
                )}
              </div>

              {/* Model selector */}
              <span className="chip">
                <Icon name="cpu" size={14} />
                <select value={model} onChange={(e) => setModel(e.target.value)}>
                  <option value="">{t('prompt.defaultModel')}</option>
                  {models.map((m) => (
                    <option key={m.modelId} value={m.modelId}>
                      {m.name}
                    </option>
                  ))}
                </select>
              </span>

              <span className="grow" />

              <button
                className="btn primary empty-send"
                disabled={!text.trim() || busy}
                onClick={() => void submit()}
                title={t('session.emptySend')}
              >
                {busy ? <span className="spinner" /> : <Icon name="send" size={16} />}
              </button>
            </div>
          </div>
          <div className="empty-hint">{t('session.emptyHint')}</div>
        </div>
      </div>
    </main>
  );
}
