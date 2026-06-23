import { useState } from 'react';
import { useStore, type SessionView } from '../../store';
import { Icon, type IconName } from '../Icon';
import { Markdown } from '../Markdown';
import { useT } from '../../i18n/useT';

/** Plan pane — the agent's current execution plan / TODO list. */
export function PlanPane({ view }: { view: SessionView }) {
  const t = useT();
  return (
    <div className="pane">
      <div className="pane-head">
        <Icon name="plan" size={15} />
        <span>{t('pane.plan')}</span>
        <span className="grow" />
        <span style={{ color: 'var(--text-faint)' }}>
          {view.plan.filter((p) => p.status === 'completed').length}/{view.plan.length}
        </span>
      </div>
      <div className="pane-body">
        {view.plan.length === 0 ? (
          <div className="empty">{t('plan.empty')}</div>
        ) : (
          <div className="plan-list">
            {view.plan.map((e, i) => (
              <div key={i} className={`plan-entry ${e.status}`}>
                <PlanMark status={e.status} />
                <span className="plan-text">{e.content}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function PlanMark({ status }: { status: string }) {
  const icon: IconName =
    status === 'completed' ? 'circle-check' : status === 'in_progress' ? 'circle-dot' : 'circle';
  return (
    <span className={`plan-mark ${status}`}>
      <Icon name={icon} size={16} />
    </span>
  );
}

/** Tasks pane — in-flight + recent tool calls (the session's "background work"). */
export function TasksPane({ view }: { view: SessionView }) {
  const t = useT();
  const tools = view.transcript.filter((t) => t.kind === 'tool') as Extract<
    SessionView['transcript'][number],
    { kind: 'tool' }
  >[];
  const running = tools.filter((t) => t.status === 'pending' || t.status === 'in_progress');
  return (
    <div className="pane">
      <div className="pane-head">
        <Icon name="tasks" size={15} />
        <span>{t('pane.tasks')}</span>
        <span className="grow" />
        <span style={{ color: 'var(--text-faint)' }}>
          {t('tasks.summary', { running: running.length, total: tools.length })}
        </span>
      </div>
      <div className="pane-body">
        {tools.length === 0 ? (
          <div className="empty">{t('tasks.empty')}</div>
        ) : (
          <div className="plan-list">
            {tools
              .slice()
              .reverse()
              .map((t) => {
                const icon: IconName =
                  t.status === 'completed'
                    ? 'circle-check'
                    : t.status === 'failed'
                      ? 'alert'
                      : 'circle-dot';
                const markClass =
                  t.status === 'completed'
                    ? 'completed'
                    : t.status === 'failed'
                      ? 'failed'
                      : 'in_progress';
                return (
                  <div key={t.id} className="plan-entry">
                    <span className={`plan-mark ${markClass}`}>
                      <Icon name={icon} size={16} spin={t.status === 'in_progress' || t.status === 'pending'} />
                    </span>
                    <span className="plan-text mono">{t.title}</span>
                  </div>
                );
              })}
          </div>
        )}
      </div>
    </div>
  );
}

/** Terminal pane — aggregated command output across the session. */
export function TerminalPane({ view }: { view: SessionView }) {
  const t = useT();
  const outputs = (
    view.transcript.filter((t) => t.kind === 'tool') as Extract<
      SessionView['transcript'][number],
      { kind: 'tool' }
    >[]
  )
    .filter((t) => t.terminalOutput)
    .map((t) => `$ ${t.title}\n${t.terminalOutput}`)
    .join('\n\n');

  return (
    <div className="pane">
      <div className="pane-head">
        <Icon name="terminal" size={15} />
        <span>{t('pane.terminal')}</span>
      </div>
      <div className="pane-body">
        <div className="console" style={{ margin: 14, minHeight: 120 }}>
          {outputs || t('terminal.noOutput')}
        </div>
      </div>
    </div>
  );
}

/** File pane — viewer and editor for files opened from tool locations. */
export function FilePane({ view }: { view: SessionView }) {
  const t = useT();
  const open = view.openFile;
  const isMarkdown = open?.path?.endsWith('.md') ?? false;
  const saveFile = useStore((s) => s.saveFile);
  const [editing, setEditing] = useState(false);
  const [editContent, setEditContent] = useState('');
  const [saving, setSaving] = useState(false);

  const handleEdit = () => {
    if (open) {
      setEditContent(open.content);
      setEditing(true);
    }
  };

  const handleSave = async () => {
    if (!open) return;
    console.log('[handleSave] Saving file:', { path: open.path, contentLength: editContent.length });
    setSaving(true);
    const success = await saveFile(view.meta.id, open.path, editContent);
    console.log('[handleSave] Save result:', success);
    setSaving(false);
    if (success) {
      setEditing(false);
    }
  };

  const handleCancel = () => {
    setEditing(false);
    setEditContent('');
  };

  return (
    <div className="pane">
      <div className="pane-head">
        <Icon name="file" size={15} />
        <span style={{ fontFamily: 'var(--mono)', flex: 1 }}>{open?.path ?? t('pane.file')}</span>
        {open && !editing && (
          <button className="icon-btn" onClick={handleEdit} title="Edit">
            <Icon name="edit" size={14} />
          </button>
        )}
        {editing && (
          <>
            <button className="icon-btn" onClick={handleCancel} title="Cancel">
              <Icon name="x" size={14} />
            </button>
            <button className="icon-btn" onClick={handleSave} disabled={saving} title="Save">
              <Icon name="check" size={14} />
            </button>
          </>
        )}
      </div>
      <div className="pane-body">
        {open ? (
          editing ? (
            <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
              <textarea
                value={editContent}
                onChange={(e) => setEditContent(e.target.value)}
                style={{
                  flex: 1,
                  margin: 0,
                  padding: 16,
                  fontFamily: 'var(--mono)',
                  fontSize: 12.5,
                  lineHeight: 1.55,
                  border: 'none',
                  resize: 'none',
                  background: 'var(--bg-sunken)',
                  color: 'var(--text)',
                }}
              />
            </div>
          ) : isMarkdown ? (
            <div style={{ padding: 16, overflow: 'auto' }}>
              <Markdown text={open.content} basePath={open.path} />
            </div>
          ) : (
            <pre style={{ margin: 0, padding: 16, fontFamily: 'var(--mono)', fontSize: 12.5, lineHeight: 1.55 }}>
              {open.content}
            </pre>
          )
        ) : (
          <div className="empty">{t('file.empty')}</div>
        )}
      </div>
    </div>
  );
}
