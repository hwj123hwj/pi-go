/**
 * TasksPanel.tsx — Tasks panel for the right sidebar.
 * Shows in-flight + recent tool calls (the session's background work).
 */

import { useStore, type SessionView as SV } from '../../store';
import { Icon, type IconName } from '../Icon';
import { useT } from '../../i18n/useT';

export function TasksPanel() {
  const activeId = useStore((s) => s.activeSessionId);
  const view = useStore((s) => (activeId ? s.sessions[activeId] : undefined));
  const t = useT();

  if (!view) {
    return (
      <div className="ws-panel">
        <div className="empty">{t('files.noProject')}</div>
      </div>
    );
  }

  const tools = view.transcript.filter((t) => t.kind === 'tool') as Extract<
    SV['transcript'][number],
    { kind: 'tool' }
  >[];
  const running = tools.filter((t) => t.status === 'pending' || t.status === 'in_progress');

  return (
    <div className="ws-panel">
      <div className="ws-panel-head">
        <Icon name="tasks" size={15} />
        <span>{t('pane.tasks')}</span>
        <span className="grow" />
        <span style={{ color: 'var(--text-faint)', fontSize: 11 }}>
          {t('tasks.summary', { running: running.length, total: tools.length })}
        </span>
      </div>
      <div className="ws-panel-list">
        {tools.length === 0 ? (
          <div className="empty">{t('tasks.empty')}</div>
        ) : (
          <div className="plan-list">
            {tools.slice().reverse().map((tool) => {
              const icon: IconName =
                tool.status === 'completed'
                  ? 'circle-check'
                  : tool.status === 'failed'
                    ? 'alert'
                    : 'circle-dot';
              const markClass =
                tool.status === 'completed'
                  ? 'completed'
                  : tool.status === 'failed'
                    ? 'failed'
                    : 'in_progress';
              return (
                <div key={tool.id} className="plan-entry">
                  <span className={`plan-mark ${markClass}`}>
                    <Icon name={icon} size={16} spin={tool.status === 'in_progress' || tool.status === 'pending'} />
                  </span>
                  <span className="plan-text mono">{tool.title}</span>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
